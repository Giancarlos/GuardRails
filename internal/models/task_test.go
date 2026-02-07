package models

import (
	"testing"
	"time"
)

func TestGenerateID(t *testing.T) {
	id := GenerateID()

	if !ValidateTaskID(id) {
		t.Errorf("GenerateID() produced invalid ID: %s", id)
	}

	if len(id) != 12 { // "gur-" + 8 hex chars
		t.Errorf("GenerateID() wrong length: got %d, want 12", len(id))
	}

	// Test uniqueness
	id2 := GenerateID()
	if id == id2 {
		t.Error("GenerateID() produced duplicate IDs")
	}
}

func TestValidateTaskID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"gur-a1b2c3d4", true},
		{"gur-00000000", true},
		{"gur-ffffffff", true},
		{"gur-a1b2c3d4.1", true},
		{"gur-a1b2c3d4.1.2", true},
		{"gur-a1b2c3d4.1.2.3", true},
		{"", false},
		{"gur-", false},
		{"gur-abc", false},        // too short
		{"gur-a1b2c3d4g", false},  // invalid hex
		{"gur-A1B2C3D4", false},   // uppercase
		{"task-a1b2c3d4", false},  // wrong prefix
		{"gur-a1b2c3d4.", false},  // trailing dot
		{"gur-a1b2c3d4.a", false}, // non-numeric subtask
	}

	for _, tt := range tests {
		got := ValidateTaskID(tt.id)
		if got != tt.valid {
			t.Errorf("ValidateTaskID(%q) = %v, want %v", tt.id, got, tt.valid)
		}
	}
}

func TestGenerateSubtaskID(t *testing.T) {
	parent := "gur-a1b2c3d4"

	subtask1 := GenerateSubtaskID(parent, 1)
	if subtask1 != "gur-a1b2c3d4.1" {
		t.Errorf("GenerateSubtaskID() = %s, want gur-a1b2c3d4.1", subtask1)
	}

	subtask2 := GenerateSubtaskID(subtask1, 2)
	if subtask2 != "gur-a1b2c3d4.1.2" {
		t.Errorf("GenerateSubtaskID() = %s, want gur-a1b2c3d4.1.2", subtask2)
	}
}

func TestGetRootID(t *testing.T) {
	tests := []struct {
		id   string
		root string
	}{
		{"gur-a1b2c3d4", "gur-a1b2c3d4"},
		{"gur-a1b2c3d4.1", "gur-a1b2c3d4"},
		{"gur-a1b2c3d4.1.2", "gur-a1b2c3d4"},
		{"gur-a1b2c3d4.1.2.3", "gur-a1b2c3d4"},
	}

	for _, tt := range tests {
		got := GetRootID(tt.id)
		if got != tt.root {
			t.Errorf("GetRootID(%q) = %q, want %q", tt.id, got, tt.root)
		}
	}
}

func TestGetParentID(t *testing.T) {
	tests := []struct {
		id     string
		parent string
	}{
		{"gur-a1b2c3d4", ""},
		{"gur-a1b2c3d4.1", "gur-a1b2c3d4"},
		{"gur-a1b2c3d4.1.2", "gur-a1b2c3d4.1"},
		{"gur-a1b2c3d4.1.2.3", "gur-a1b2c3d4.1.2"},
	}

	for _, tt := range tests {
		got := GetParentID(tt.id)
		if got != tt.parent {
			t.Errorf("GetParentID(%q) = %q, want %q", tt.id, got, tt.parent)
		}
	}
}

func TestGetDepth(t *testing.T) {
	tests := []struct {
		id    string
		depth int
	}{
		{"gur-a1b2c3d4", 0},
		{"gur-a1b2c3d4.1", 1},
		{"gur-a1b2c3d4.1.2", 2},
		{"gur-a1b2c3d4.1.2.3", 3},
	}

	for _, tt := range tests {
		got := GetDepth(tt.id)
		if got != tt.depth {
			t.Errorf("GetDepth(%q) = %d, want %d", tt.id, got, tt.depth)
		}
	}
}

func TestTaskClose(t *testing.T) {
	task := &Task{
		ID:     "gur-a1b2c3d4",
		Status: StatusOpen,
	}

	task.Close("completed")

	if task.Status != StatusClosed {
		t.Errorf("Close() status = %s, want %s", task.Status, StatusClosed)
	}
	if task.CloseReason != "completed" {
		t.Errorf("Close() reason = %s, want completed", task.CloseReason)
	}
	if task.ClosedAt == nil {
		t.Error("Close() did not set ClosedAt")
	}
}

func TestTaskReopen(t *testing.T) {
	now := time.Now()
	task := &Task{
		ID:          "gur-a1b2c3d4",
		Status:      StatusClosed,
		CloseReason: "done",
		ClosedAt:    &now,
	}

	task.Reopen()

	if task.Status != StatusOpen {
		t.Errorf("Reopen() status = %s, want %s", task.Status, StatusOpen)
	}
	if task.CloseReason != "" {
		t.Errorf("Reopen() reason = %s, want empty", task.CloseReason)
	}
	if task.ClosedAt != nil {
		t.Error("Reopen() did not clear ClosedAt")
	}
}

func TestTaskIsClosed(t *testing.T) {
	tests := []struct {
		status string
		closed bool
	}{
		{StatusOpen, false},
		{StatusInProgress, false},
		{StatusClosed, true},
		{StatusArchived, false},
	}

	for _, tt := range tests {
		task := &Task{Status: tt.status}
		if got := task.IsClosed(); got != tt.closed {
			t.Errorf("IsClosed() with status %s = %v, want %v", tt.status, got, tt.closed)
		}
	}
}

func TestTaskLabels(t *testing.T) {
	task := &Task{ID: "gur-a1b2c3d4"}

	// Add labels
	task.AddLabel("bug")
	task.AddLabel("urgent")

	if len(task.Labels) != 2 {
		t.Errorf("AddLabel() count = %d, want 2", len(task.Labels))
	}

	// Add duplicate - should be ignored
	task.AddLabel("bug")
	if len(task.Labels) != 2 {
		t.Errorf("AddLabel() duplicate not ignored, count = %d", len(task.Labels))
	}

	// Remove label
	task.RemoveLabel("bug")
	if len(task.Labels) != 1 {
		t.Errorf("RemoveLabel() count = %d, want 1", len(task.Labels))
	}

	// Remove non-existent - should be no-op
	task.RemoveLabel("nonexistent")
	if len(task.Labels) != 1 {
		t.Errorf("RemoveLabel() non-existent changed count to %d", len(task.Labels))
	}
}

func TestTaskPriorityString(t *testing.T) {
	tests := []struct {
		priority int
		expected string
	}{
		{PriorityCritical, "P0 (Critical)"},
		{PriorityHigh, "P1 (High)"},
		{PriorityMedium, "P2 (Medium)"},
		{PriorityLow, "P3 (Low)"},
		{PriorityLowest, "P4 (Lowest)"},
		{99, "Unknown"},
	}

	for _, tt := range tests {
		task := &Task{Priority: tt.priority}
		if got := task.PriorityString(); got != tt.expected {
			t.Errorf("PriorityString() with %d = %s, want %s", tt.priority, got, tt.expected)
		}
	}
}

func TestTaskCompact(t *testing.T) {
	task := &Task{
		ID:          "gur-a1b2c3d4",
		Title:       "Fix bug",
		Description: "Long description here",
		Notes:       "Some notes",
		Type:        TypeBug,
		CloseReason: "Fixed in commit abc",
	}

	task.Compact()

	if !task.Compacted {
		t.Error("Compact() did not set Compacted flag")
	}
	if task.Description != "" {
		t.Error("Compact() did not clear Description")
	}
	if task.Notes != "" {
		t.Error("Compact() did not clear Notes")
	}
	if task.Summary == "" {
		t.Error("Compact() did not generate Summary")
	}

	// Compact again should be no-op
	task.Summary = "modified"
	task.Compact()
	if task.Summary != "modified" {
		t.Error("Compact() modified already compacted task")
	}
}

func TestTaskArchive(t *testing.T) {
	task := &Task{Status: StatusClosed}

	task.Archive()
	if task.Status != StatusArchived {
		t.Errorf("Archive() status = %s, want %s", task.Status, StatusArchived)
	}

	if !task.IsArchived() {
		t.Error("IsArchived() = false after Archive()")
	}

	task.Unarchive()
	if task.Status != StatusClosed {
		t.Errorf("Unarchive() status = %s, want %s", task.Status, StatusClosed)
	}
}

func TestTaskAppendNotes(t *testing.T) {
	task := &Task{ID: "gur-a1b2c3d4"}

	task.AppendNotes("First note")
	if task.Notes == "" {
		t.Error("AppendNotes() did not add note")
	}

	task.AppendNotes("Second note")
	if len(task.Notes) <= len("First note") {
		t.Error("AppendNotes() did not append second note")
	}
}
