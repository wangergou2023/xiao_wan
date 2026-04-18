package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
)

func TestCreateAIReqAddsNativeTools(t *testing.T) {
	originalConfig := vars.APIConfig
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
	})

	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.Provider = "bigmodel"
	vars.APIConfig.Knowledge.Model = "glm-5.1"
	vars.APIConfig.Knowledge.SaveChat = false

	req := CreateAIReq("回家充电去", "test-esn", false, false)
	if len(req.Tools) != len(nativeToolDefinitions) {
		t.Fatalf("expected %d native tools, got %d", len(nativeToolDefinitions), len(req.Tools))
	}
	if req.ToolChoice != "auto" {
		t.Fatalf("expected tool_choice auto, got %#v", req.ToolChoice)
	}
	if req.ParallelToolCalls != false {
		t.Fatalf("expected parallel tool calls disabled, got %#v", req.ParallelToolCalls)
	}
}

func TestStreamedToolCallAccumulatorMergesChunks(t *testing.T) {
	acc := streamedToolCallAccumulator{}
	idx := 0

	acc.AddDelta([]openai.ToolCall{{
		Index: &idx,
		ID:    "call_1",
		Type:  openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name: "go",
		},
	}})
	acc.AddDelta([]openai.ToolCall{{
		Index: &idx,
		Function: openai.FunctionCall{
			Name:      "Charge",
			Arguments: "{}",
		},
	}})

	calls := acc.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Function.Name != "goCharge" {
		t.Fatalf("expected merged function name goCharge, got %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != "{}" {
		t.Fatalf("expected merged arguments, got %q", calls[0].Function.Arguments)
	}
}

func TestExecuteNativeToolCallsFileTools(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	toolCalls := []openai.ToolCall{
		{
			ID:   "write_1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "writeFile",
				Arguments: `{"path":"notes/test.txt","content":"hello tool world\nline 2","mode":"overwrite","overwrite":true}`,
			},
		},
		{
			ID:   "read_1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "readFile",
				Arguments: `{"path":"notes/test.txt","mode":"bytes","offset":0,"length":5}`,
			},
		},
		{
			ID:   "read_2",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "readFile",
				Arguments: `{"path":"notes/test.txt","mode":"lines","start_line":2,"max_lines":1}`,
			},
		},
		{
			ID:   "list_1",
			Type: openai.ToolTypeFunction,
			Function: openai.FunctionCall{
				Name:      "listFiles",
				Arguments: `{"path":"notes"}`,
			},
		},
	}

	deferred, results, needFollowUp := executeNativeToolCalls(toolCalls, nativeToolContext{})
	if len(deferred) != 0 {
		t.Fatalf("expected no deferred actions, got %d", len(deferred))
	}
	if !needFollowUp {
		t.Fatalf("expected file tools to request a follow-up response")
	}
	if len(results) != 4 {
		t.Fatalf("expected 4 tool results, got %d", len(results))
	}

	data, err := os.ReadFile(filepath.Join(tmp, "notes", "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello tool world\nline 2" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
	if !strings.Contains(results[1].Content, "hello") {
		t.Fatalf("expected byte read result to contain first bytes, got %s", results[1].Content)
	}
	if !strings.Contains(results[1].Content, `"state":"truncated"`) {
		t.Fatalf("expected byte read result to mark truncated state, got %s", results[1].Content)
	}
	if !strings.Contains(results[2].Content, "2|line 2") {
		t.Fatalf("expected line read result to contain numbered line, got %s", results[2].Content)
	}
	if !strings.Contains(results[3].Content, "test.txt") {
		t.Fatalf("expected listFiles result to contain file name, got %s", results[3].Content)
	}
}

func TestWriteFileRequiresOverwriteForExistingFile(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	if err := os.MkdirAll("notes", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("notes/existing.txt", []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	toolCalls := []openai.ToolCall{{
		ID:   "write_fail",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "writeFile",
			Arguments: `{"path":"notes/existing.txt","content":"new","mode":"overwrite"}`,
		},
	}}

	_, results, _ := executeNativeToolCalls(toolCalls, nativeToolContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	if !strings.Contains(results[0].Content, "overwrite=true") {
		t.Fatalf("expected overwrite guidance, got %s", results[0].Content)
	}
}

func TestEditFileToolEditsUniqueMatch(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	if err := os.MkdirAll("notes", 0o755); err != nil {
		t.Fatal(err)
	}
	original := `hello
replace me once
bye
`
	if err := os.WriteFile("notes/edit.txt", []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	toolCalls := []openai.ToolCall{{
		ID:   "edit_1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "editFile",
			Arguments: `{"path":"notes/edit.txt","old_text":"replace me once","new_text":"edited text"}`,
		},
	}}

	_, results, needFollowUp := executeNativeToolCalls(toolCalls, nativeToolContext{})
	if !needFollowUp {
		t.Fatalf("expected edit tool to request a follow-up response")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	data, err := os.ReadFile(filepath.Join(tmp, "notes", "edit.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `hello
edited text
bye
` {
		t.Fatalf("unexpected edited file contents: %q", string(data))
	}
	if !strings.Contains(results[0].Content, `"status":"ok"`) {
		t.Fatalf("expected successful edit result, got %s", results[0].Content)
	}
	if !strings.Contains(results[0].Content, `"state":"complete"`) {
		t.Fatalf("expected successful edit result to mark complete state, got %s", results[0].Content)
	}
}

func TestEditFileToolRejectsAmbiguousMatch(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	if err := os.MkdirAll("notes", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("notes/dup.txt", []byte(`same
same
`), 0o644); err != nil {
		t.Fatal(err)
	}

	toolCalls := []openai.ToolCall{{
		ID:   "edit_fail",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "editFile",
			Arguments: `{"path":"notes/dup.txt","old_text":"same","new_text":"new"}`,
		},
	}}

	_, results, _ := executeNativeToolCalls(toolCalls, nativeToolContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	if !strings.Contains(results[0].Content, "appears 2 times") {
		t.Fatalf("expected ambiguity warning, got %s", results[0].Content)
	}
}

func TestResolveSafeToolPathRejectsSymlinkEscape(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("notes", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideFile, filepath.Join("notes", "secret-link.txt")); err != nil {
		t.Fatal(err)
	}

	_, err = resolveSafeToolPath(filepath.Join("notes", "secret-link.txt"), false)
	if err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestExecuteNativeToolCallsRunCommand(t *testing.T) {
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWD)

	toolCalls := []openai.ToolCall{{
		ID:   "cmd_1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "runCommand",
			Arguments: `{"command":"pwd"}`,
		},
	}}

	_, results, needFollowUp := executeNativeToolCalls(toolCalls, nativeToolContext{})
	if !needFollowUp {
		t.Fatalf("expected command tool to request a follow-up response")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	if !strings.Contains(results[0].Content, tmp) {
		t.Fatalf("expected pwd output to mention temp dir, got %s", results[0].Content)
	}
	if !strings.Contains(results[0].Content, `"state":"complete"`) {
		t.Fatalf("expected pwd result to mark complete state, got %s", results[0].Content)
	}
}

func TestExecuteNativeToolCallsScheduledRobotTool(t *testing.T) {
	toolCalls := []openai.ToolCall{{
		ID:   "charge_1",
		Type: openai.ToolTypeFunction,
		Function: openai.FunctionCall{
			Name:      "goCharge",
			Arguments: `{}`,
		},
	}}

	deferred, results, needFollowUp := executeNativeToolCalls(toolCalls, nativeToolContext{})
	if len(deferred) != 1 {
		t.Fatalf("expected 1 deferred action, got %d", len(deferred))
	}
	if needFollowUp {
		t.Fatalf("expected scheduled robot action to skip follow-up")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(results))
	}
	if !strings.Contains(results[0].Content, `"status":"scheduled"`) {
		t.Fatalf("expected scheduled status, got %s", results[0].Content)
	}
	if !strings.Contains(results[0].Content, `"state":"pending"`) {
		t.Fatalf("expected pending state, got %s", results[0].Content)
	}
}
