package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Giancarlos/guardrails/internal/models"
)

// Formatter defines the interface for output formatting
type Formatter interface {
	Task(t *models.Task)
	TaskList(tasks []models.Task, title string)
	TaskBrief(t *models.Task)
	Gate(g *models.Gate)
	GateList(gates []models.Gate)
	Success(msg string)
	Error(err error)
	Info(msg string)
	KeyValue(key, value string)
	Section(title string)
	JSON(v interface{})
}

// TextFormatter outputs human-readable text
type TextFormatter struct{}

// CompactFormatter outputs token-optimized text for AI agents
type CompactFormatter struct{}

// JSONFormatter outputs JSON
type JSONFormatter struct{}

// New returns the appropriate formatter based on output flags
func New(jsonOutput bool) Formatter {
	if jsonOutput {
		return &JSONFormatter{}
	}
	if IsCompact() {
		return &CompactFormatter{}
	}
	return &TextFormatter{}
}

// compact tracks whether compact output mode is enabled
var compact bool

// SetCompact enables or disables compact output mode
func SetCompact(v bool) { compact = v }

// IsCompact returns whether compact output mode is active
func IsCompact() bool { return compact }

// TextFormatter implementations

func (f *TextFormatter) Task(t *models.Task) {
	fmt.Printf("ID:       %s\n", t.ID)
	if t.ParentID != "" {
		fmt.Printf("Parent:   %s\n", t.ParentID)
	}
	fmt.Printf("Title:    %s\n", t.Title)
	fmt.Printf("Status:   %s\n", t.Status)
	fmt.Printf("Priority: %s\n", t.PriorityString())
	fmt.Printf("Type:     %s\n", t.Type)
	if t.Description != "" {
		fmt.Printf("Desc:     %s\n", t.Description)
	}
	if t.Assignee != "" {
		fmt.Printf("Assignee: %s\n", t.Assignee)
	}
	if len(t.Labels) > 0 {
		fmt.Printf("Labels:   %v\n", t.Labels)
	}
	if t.Summary != "" {
		fmt.Printf("Summary:  %s\n", t.Summary)
	}
	fmt.Printf("Created:  %s\n", t.CreatedAt.Format(models.DateTimeShortFormat))
}

func (f *TextFormatter) TaskList(tasks []models.Task, title string) {
	if title != "" {
		fmt.Printf("%s (%d):\n", title, len(tasks))
	}
	for _, t := range tasks {
		f.TaskBrief(&t)
	}
}

func (f *TextFormatter) TaskBrief(t *models.Task) {
	indent := ""
	if strings.Contains(t.ID, ".") {
		indent = "  "
	}
	typeStr := ""
	if t.Type != models.TypeTask {
		typeStr = fmt.Sprintf(" (%s)", t.Type)
	}
	fmt.Printf("%s[%s] P%d %s - %s%s\n", indent, t.ID, t.Priority, t.Status, t.Title, typeStr)
}

func (f *TextFormatter) Gate(g *models.Gate) {
	fmt.Printf("ID:       %s\n", g.ID)
	fmt.Printf("Title:    %s\n", g.Title)
	fmt.Printf("Type:     %s\n", g.TypeString())
	fmt.Printf("Priority: P%d\n", g.Priority)
	fmt.Printf("Result:   %s\n", g.ResultString())
	if g.Category != "" {
		fmt.Printf("Category: %s\n", g.Category)
	}
	if g.Description != "" {
		fmt.Printf("Desc:     %s\n", g.Description)
	}
}

func (f *TextFormatter) GateList(gates []models.Gate) {
	for _, g := range gates {
		cat := ""
		if g.Category != "" {
			cat = "[" + g.Category + "] "
		}
		fmt.Printf("[%s] %s%s - %s (%s)\n", g.ID, cat, g.ResultString(), g.Title, g.TypeString())
	}
}

func (f *TextFormatter) Success(msg string) {
	fmt.Println(msg)
}

func (f *TextFormatter) Error(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
}

func (f *TextFormatter) Info(msg string) {
	fmt.Println(msg)
}

func (f *TextFormatter) KeyValue(key, value string) {
	fmt.Printf("%s: %s\n", key, value)
}

func (f *TextFormatter) Section(title string) {
	fmt.Printf("\n%s:\n", title)
}

func (f *TextFormatter) JSON(v interface{}) {
	// TextFormatter doesn't output JSON, but provide fallback
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		f.Error(err)
		return
	}
	fmt.Println(string(data))
}

// CompactFormatter implementations
// Designed to minimize token usage while preserving all actionable information.

func (f *CompactFormatter) Task(t *models.Task) {
	// Single-line core info, then only non-empty extras
	parts := []string{t.ID, "P" + fmt.Sprint(t.Priority), t.Status, t.Title}
	if t.Type != models.TypeTask {
		parts = append(parts, "("+t.Type+")")
	}
	fmt.Println(strings.Join(parts, " | "))
	if t.ParentID != "" {
		fmt.Printf("parent:%s\n", t.ParentID)
	}
	if t.Description != "" {
		fmt.Printf("desc:%s\n", t.Description)
	}
	if t.Assignee != "" {
		fmt.Printf("assignee:%s\n", t.Assignee)
	}
	if len(t.Labels) > 0 {
		fmt.Printf("labels:%s\n", strings.Join(t.Labels, ","))
	}
}

func (f *CompactFormatter) TaskList(tasks []models.Task, title string) {
	if title != "" {
		fmt.Printf("%s (%d):\n", title, len(tasks))
	}
	for _, t := range tasks {
		f.TaskBrief(&t)
	}
}

func (f *CompactFormatter) TaskBrief(t *models.Task) {
	indent := ""
	if strings.Contains(t.ID, ".") {
		indent = "  "
	}
	typeStr := ""
	if t.Type != models.TypeTask {
		typeStr = " (" + t.Type + ")"
	}
	fmt.Printf("%s%s P%d %s %s%s\n", indent, t.ID, t.Priority, t.Status, t.Title, typeStr)
}

func (f *CompactFormatter) Gate(g *models.Gate) {
	parts := []string{g.ID, g.ResultString(), g.Title, g.TypeString()}
	if g.Category != "" {
		parts = append(parts, "["+g.Category+"]")
	}
	fmt.Println(strings.Join(parts, " | "))
}

func (f *CompactFormatter) GateList(gates []models.Gate) {
	for _, g := range gates {
		cat := ""
		if g.Category != "" {
			cat = "[" + g.Category + "] "
		}
		fmt.Printf("%s %s%s %s (%s)\n", g.ID, cat, g.ResultString(), g.Title, g.TypeString())
	}
}

func (f *CompactFormatter) Success(msg string) {
	fmt.Printf("ok: %s\n", msg)
}

func (f *CompactFormatter) Error(err error) {
	fmt.Fprintf(os.Stderr, "err: %v\n", err)
}

func (f *CompactFormatter) Info(msg string) {
	fmt.Println(msg)
}

func (f *CompactFormatter) KeyValue(key, value string) {
	fmt.Printf("%s:%s\n", key, value)
}

func (f *CompactFormatter) Section(title string) {
	// No section headers in compact mode
}

func (f *CompactFormatter) JSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		f.Error(err)
		return
	}
	fmt.Println(string(data))
}

// JSONFormatter implementations

func (f *JSONFormatter) Task(t *models.Task) {
	f.JSON(t)
}

func (f *JSONFormatter) TaskList(tasks []models.Task, title string) {
	f.JSON(map[string]interface{}{
		"count": len(tasks),
		"tasks": tasks,
	})
}

func (f *JSONFormatter) TaskBrief(t *models.Task) {
	f.JSON(t)
}

func (f *JSONFormatter) Gate(g *models.Gate) {
	f.JSON(g)
}

func (f *JSONFormatter) GateList(gates []models.Gate) {
	f.JSON(map[string]interface{}{
		"count": len(gates),
		"gates": gates,
	})
}

func (f *JSONFormatter) Success(msg string) {
	f.JSON(map[string]interface{}{"success": true, "message": msg})
}

func (f *JSONFormatter) Error(err error) {
	f.JSON(map[string]interface{}{"error": true, "message": err.Error()})
}

func (f *JSONFormatter) Info(msg string) {
	f.JSON(map[string]interface{}{"message": msg})
}

func (f *JSONFormatter) KeyValue(key, value string) {
	f.JSON(map[string]string{key: value})
}

func (f *JSONFormatter) Section(title string) {
	// JSON doesn't need section headers
}

func (f *JSONFormatter) JSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, `{"error": true, "message": "JSON marshal error: %s"}`+"\n", err.Error())
		return
	}
	fmt.Println(string(data))
}
