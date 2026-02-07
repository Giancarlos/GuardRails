package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"guardrails/internal/models"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestNewFormatter(t *testing.T) {
	textFormatter := New(false)
	if _, ok := textFormatter.(*TextFormatter); !ok {
		t.Error("New(false) should return TextFormatter")
	}

	jsonFormatter := New(true)
	if _, ok := jsonFormatter.(*JSONFormatter); !ok {
		t.Error("New(true) should return JSONFormatter")
	}
}

func TestTextFormatterTask(t *testing.T) {
	f := &TextFormatter{}
	task := &models.Task{
		ID:          "gur-test123",
		Title:       "Test Task",
		Status:      models.StatusOpen,
		Priority:    models.PriorityHigh,
		Type:        models.TypeBug,
		Description: "Test description",
	}

	output := captureOutput(func() {
		f.Task(task)
	})

	if !strings.Contains(output, "gur-test123") {
		t.Error("output should contain task ID")
	}
	if !strings.Contains(output, "Test Task") {
		t.Error("output should contain task title")
	}
	if !strings.Contains(output, "open") {
		t.Error("output should contain status")
	}
}

func TestTextFormatterTaskBrief(t *testing.T) {
	f := &TextFormatter{}

	// Regular task
	task := &models.Task{
		ID:       "gur-abc123",
		Title:    "Regular task",
		Status:   models.StatusOpen,
		Priority: 1,
		Type:     models.TypeTask,
	}

	output := captureOutput(func() {
		f.TaskBrief(task)
	})

	if !strings.Contains(output, "[gur-abc123]") {
		t.Error("output should contain task ID in brackets")
	}
	if !strings.Contains(output, "P1") {
		t.Error("output should contain priority")
	}
	if strings.Contains(output, "(task)") {
		t.Error("regular tasks should not show type")
	}

	// Bug type should show
	bug := &models.Task{
		ID:       "gur-bug123",
		Title:    "Bug task",
		Status:   models.StatusOpen,
		Priority: 0,
		Type:     models.TypeBug,
	}

	output = captureOutput(func() {
		f.TaskBrief(bug)
	})

	if !strings.Contains(output, "(bug)") {
		t.Error("bug type should be shown")
	}

	// Subtask should be indented
	subtask := &models.Task{
		ID:       "gur-parent.1",
		Title:    "Subtask",
		Status:   models.StatusOpen,
		Priority: 2,
		Type:     models.TypeTask,
	}

	output = captureOutput(func() {
		f.TaskBrief(subtask)
	})

	if !strings.HasPrefix(output, "  ") {
		t.Error("subtask should be indented")
	}
}

func TestTextFormatterSuccess(t *testing.T) {
	f := &TextFormatter{}

	output := captureOutput(func() {
		f.Success("Operation completed")
	})

	if !strings.Contains(output, "Operation completed") {
		t.Errorf("output = %q, want to contain 'Operation completed'", output)
	}
}

func TestJSONFormatterTask(t *testing.T) {
	f := &JSONFormatter{}
	task := &models.Task{
		ID:       "gur-json123",
		Title:    "JSON Task",
		Status:   models.StatusClosed,
		Priority: 2,
		Type:     models.TypeFeature,
	}

	output := captureOutput(func() {
		f.Task(task)
	})

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["id"] != "gur-json123" {
		t.Errorf("id = %v, want gur-json123", result["id"])
	}
	if result["title"] != "JSON Task" {
		t.Errorf("title = %v, want JSON Task", result["title"])
	}
}

func TestJSONFormatterTaskList(t *testing.T) {
	f := &JSONFormatter{}
	tasks := []models.Task{
		{ID: "gur-1", Title: "Task 1"},
		{ID: "gur-2", Title: "Task 2"},
	}

	output := captureOutput(func() {
		f.TaskList(tasks, "Test Tasks")
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["count"].(float64) != 2 {
		t.Errorf("count = %v, want 2", result["count"])
	}

	tasksList, ok := result["tasks"].([]interface{})
	if !ok {
		t.Fatal("tasks should be an array")
	}
	if len(tasksList) != 2 {
		t.Errorf("tasks length = %d, want 2", len(tasksList))
	}
}

func TestJSONFormatterSuccess(t *testing.T) {
	f := &JSONFormatter{}

	output := captureOutput(func() {
		f.Success("Done!")
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["success"] != true {
		t.Errorf("success = %v, want true", result["success"])
	}
	if result["message"] != "Done!" {
		t.Errorf("message = %v, want 'Done!'", result["message"])
	}
}

func TestJSONFormatterError(t *testing.T) {
	f := &JSONFormatter{}

	output := captureOutput(func() {
		f.Error(io.EOF)
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["error"] != true {
		t.Errorf("error = %v, want true", result["error"])
	}
	if result["message"] != "EOF" {
		t.Errorf("message = %v, want 'EOF'", result["message"])
	}
}

func TestTextFormatterGate(t *testing.T) {
	f := &TextFormatter{}
	gate := &models.Gate{
		ID:          "gate-12345678",
		Title:       "Test Gate",
		Type:        "manual",
		Priority:    1,
		LastResult:  models.GatePassed,
		Category:    "testing",
		Description: "Gate description",
	}

	output := captureOutput(func() {
		f.Gate(gate)
	})

	if !strings.Contains(output, "gate-123") {
		t.Error("output should contain gate ID")
	}
	if !strings.Contains(output, "Test Gate") {
		t.Error("output should contain gate title")
	}
	if !strings.Contains(output, "testing") {
		t.Error("output should contain category")
	}
}
