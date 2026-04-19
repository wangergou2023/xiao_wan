package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wangergou2023/xiao_wan/chipper/pkg/vars"
	workspacepkg "github.com/wangergou2023/xiao_wan/chipper/pkg/xiaowan/workspace"
)

func TestCreateAIReqAddsTodoPromptForComplexTask(t *testing.T) {
	originalConfig := vars.APIConfig
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldWirepodHome := os.Getenv("WIREPOD_HOME")
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WIREPOD_HOME", tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
		_ = os.Chdir(oldWD)
		_ = os.Setenv("WIREPOD_HOME", oldWirepodHome)
	})

	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.Provider = "bigmodel"
	vars.APIConfig.Knowledge.Model = "glm-5.1"
	vars.APIConfig.Knowledge.SaveChat = false

	req := CreateAIReq("帮我记住我喜欢吃苹果并更新长期记忆", "todo-esn", false, false)
	found := false
	for _, msg := range req.Messages {
		if msg.Role != "system" {
			continue
		}
		if strings.Contains(msg.Content, "This request looks like multi-step work") && strings.Contains(msg.Content, "todo_write") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected todo planning system prompt, got %#v", req.Messages)
	}
}

func TestCreateAIReqAddsResumePromptWhenTodoExists(t *testing.T) {
	originalConfig := vars.APIConfig
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	oldWirepodHome := os.Getenv("WIREPOD_HOME")
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("WIREPOD_HOME", tmp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		vars.APIConfig = originalConfig
		_ = os.Chdir(oldWD)
		_ = os.Setenv("WIREPOD_HOME", oldWirepodHome)
	})

	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.Provider = "bigmodel"
	vars.APIConfig.Knowledge.Model = "glm-5.1"
	vars.APIConfig.Knowledge.SaveChat = false

	if err := os.MkdirAll(filepath.Join(tmp, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := workspacepkg.SaveTodoState(workspacepkg.TodoState{
		ActivePlan: &workspacepkg.TodoPlan{
			Goal:   "更新记忆",
			Status: "in_progress",
			Steps: []workspacepkg.TodoStep{
				{ID: "read_memory", Text: "读取 MEMORY.md", Status: "completed"},
				{ID: "write_memory", Text: "写入偏好", Status: "in_progress"},
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	req := CreateAIReq("继续刚才那个任务", "todo-esn", false, false)
	found := false
	for _, msg := range req.Messages {
		if msg.Role != "system" {
			continue
		}
		if strings.Contains(msg.Content, "There is already an active plan") && strings.Contains(msg.Content, "todo_read") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected resume todo planning system prompt, got %#v", req.Messages)
	}
}
