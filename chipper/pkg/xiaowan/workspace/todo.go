package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	TodoJSONRelativePath = "state/todo.json"
	TodoMarkdownPath     = "TODO.md"
	maxTodoHistoryPlans  = 12
)

type TodoState struct {
	ActivePlan  *TodoPlan  `json:"active_plan,omitempty"`
	RecentPlans []TodoPlan `json:"recent_plans,omitempty"`
	UpdatedAt   string     `json:"updated_at,omitempty"`
}

type TodoPlan struct {
	Goal      string     `json:"goal"`
	Status    string     `json:"status"`
	Steps     []TodoStep `json:"steps"`
	CreatedAt string     `json:"created_at,omitempty"`
	UpdatedAt string     `json:"updated_at,omitempty"`
}

type TodoStep struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
	Notes  string `json:"notes,omitempty"`
}

func LoadTodoState() (TodoState, error) {
	for _, root := range WorkspaceRoots() {
		path := filepath.Join(root, TodoJSONRelativePath)
		data, err := os.ReadFile(path)
		if err == nil {
			var state TodoState
			if err := json.Unmarshal(data, &state); err != nil {
				return TodoState{}, err
			}
			normalizeTodoState(&state)
			return state, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return TodoState{}, err
		}
	}
	return TodoState{}, nil
}

func SaveTodoState(state TodoState) error {
	normalizeTodoState(&state)
	now := time.Now().Format(time.RFC3339)
	state.UpdatedAt = now
	if state.ActivePlan != nil {
		if strings.TrimSpace(state.ActivePlan.CreatedAt) == "" {
			state.ActivePlan.CreatedAt = now
		}
		state.ActivePlan.UpdatedAt = now
	}
	for i := range state.RecentPlans {
		normalizeTodoPlan(&state.RecentPlans[i])
		if strings.TrimSpace(state.RecentPlans[i].UpdatedAt) == "" {
			state.RecentPlans[i].UpdatedAt = now
		}
	}
	if len(state.RecentPlans) > maxTodoHistoryPlans {
		state.RecentPlans = append([]TodoPlan(nil), state.RecentPlans[len(state.RecentPlans)-maxTodoHistoryPlans:]...)
	}

	jsonPath := ResolveWritableDocPath(TodoJSONRelativePath)
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(jsonPath, data, 0o644); err != nil {
		return err
	}

	mdPath := ResolveWritableDocPath(TodoMarkdownPath)
	if err := os.MkdirAll(filepath.Dir(mdPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(mdPath, []byte(renderTodoMarkdown(state)), 0o644)
}

func BuildTodoPromptContext() string {
	state, err := LoadTodoState()
	if err != nil || state.ActivePlan == nil {
		return ""
	}
	plan := state.ActivePlan
	var b strings.Builder
	b.WriteString("workspace/TODO.md:\n")
	b.WriteString("Goal: " + strings.TrimSpace(plan.Goal) + "\n")
	b.WriteString("Status: " + strings.TrimSpace(plan.Status))
	for _, step := range plan.Steps {
		text := strings.TrimSpace(step.Text)
		if text == "" {
			continue
		}
		b.WriteString("\n- [" + strings.TrimSpace(step.Status) + "] ")
		if id := strings.TrimSpace(step.ID); id != "" {
			b.WriteString(id + ": ")
		}
		b.WriteString(text)
		if notes := strings.TrimSpace(step.Notes); notes != "" {
			b.WriteString(" (" + notes + ")")
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeTodoState(state *TodoState) {
	if state == nil {
		return
	}
	for i := range state.RecentPlans {
		normalizeTodoPlan(&state.RecentPlans[i])
	}
	if state.ActivePlan == nil {
		if len(state.RecentPlans) > maxTodoHistoryPlans {
			state.RecentPlans = append([]TodoPlan(nil), state.RecentPlans[len(state.RecentPlans)-maxTodoHistoryPlans:]...)
		}
		return
	}
	normalizeTodoPlan(state.ActivePlan)
	if len(state.RecentPlans) > maxTodoHistoryPlans {
		state.RecentPlans = append([]TodoPlan(nil), state.RecentPlans[len(state.RecentPlans)-maxTodoHistoryPlans:]...)
	}
}

func NormalizeTodoStateForRuntime(state *TodoState) {
	normalizeTodoState(state)
}

func normalizeTodoPlan(plan *TodoPlan) {
	if plan == nil {
		return
	}
	steps := make([]TodoStep, 0, len(plan.Steps))
	for i, step := range plan.Steps {
		step.ID = normalizeTodoStepID(step.ID, i)
		step.Text = strings.TrimSpace(step.Text)
		step.Status = normalizeTodoStatus(step.Status)
		step.Notes = strings.TrimSpace(step.Notes)
		if step.Text == "" {
			continue
		}
		steps = append(steps, step)
	}
	plan.Goal = strings.TrimSpace(plan.Goal)
	plan.Status = normalizeTodoStatus(plan.Status)
	plan.Steps = steps
	recalcTodoPlanStatus(plan)
}

func ArchiveActivePlan(state *TodoState) bool {
	if state == nil || state.ActivePlan == nil {
		return false
	}
	normalizeTodoPlan(state.ActivePlan)
	archived := *state.ActivePlan
	if strings.TrimSpace(archived.UpdatedAt) == "" {
		archived.UpdatedAt = time.Now().Format(time.RFC3339)
	}
	state.RecentPlans = append(state.RecentPlans, archived)
	if len(state.RecentPlans) > maxTodoHistoryPlans {
		state.RecentPlans = append([]TodoPlan(nil), state.RecentPlans[len(state.RecentPlans)-maxTodoHistoryPlans:]...)
	}
	state.ActivePlan = nil
	return true
}

func normalizeTodoStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "":
		return "pending"
	case "in_progress", "in-progress", "doing", "active":
		return "in_progress"
	case "completed", "done", "finished":
		return "completed"
	case "cancelled", "canceled", "abandoned":
		return "cancelled"
	default:
		return "pending"
	}
}

func normalizeTodoStepID(id string, index int) string {
	id = strings.TrimSpace(strings.ToLower(id))
	if id != "" {
		replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", "\\", "_", ".", "_")
		id = replacer.Replace(id)
		return id
	}
	return fmt.Sprintf("step_%d", index+1)
}

func recalcTodoPlanStatus(plan *TodoPlan) {
	if plan == nil {
		return
	}
	if len(plan.Steps) == 0 {
		plan.Status = normalizeTodoStatus(plan.Status)
		return
	}
	var completed, cancelled, inProgress int
	for _, step := range plan.Steps {
		switch normalizeTodoStatus(step.Status) {
		case "completed":
			completed++
		case "cancelled":
			cancelled++
		case "in_progress":
			inProgress++
		}
	}
	switch {
	case completed == len(plan.Steps):
		plan.Status = "completed"
	case completed+cancelled == len(plan.Steps) && len(plan.Steps) > 0:
		plan.Status = "cancelled"
	case inProgress > 0:
		plan.Status = "in_progress"
	default:
		plan.Status = "pending"
	}
}

func renderTodoMarkdown(state TodoState) string {
	var b strings.Builder
	b.WriteString("# Todo\n\n")
	if state.ActivePlan == nil {
		b.WriteString("No active plan.\n")
		return b.String()
	}
	plan := state.ActivePlan
	b.WriteString("- Goal: " + strings.TrimSpace(plan.Goal) + "\n")
	b.WriteString("- Status: " + strings.TrimSpace(plan.Status) + "\n")
	if strings.TrimSpace(plan.UpdatedAt) != "" {
		b.WriteString("- Updated: " + strings.TrimSpace(plan.UpdatedAt) + "\n")
	}
	b.WriteString("\n## Steps\n")
	for _, step := range plan.Steps {
		if strings.TrimSpace(step.Text) == "" {
			continue
		}
		box := "[ ]"
		switch normalizeTodoStatus(step.Status) {
		case "completed":
			box = "[x]"
		case "in_progress":
			box = "[~]"
		case "cancelled":
			box = "[-]"
		}
		b.WriteString("- " + box + " ")
		if id := strings.TrimSpace(step.ID); id != "" {
			b.WriteString(id + ": ")
		}
		b.WriteString(strings.TrimSpace(step.Text))
		if notes := strings.TrimSpace(step.Notes); notes != "" {
			b.WriteString(" (" + notes + ")")
		}
		b.WriteString("\n")
	}
	return b.String()
}
