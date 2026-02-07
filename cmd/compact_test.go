package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

func TestCompactTask(t *testing.T) {
	// Setup temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("GUR_DB_PATH", dbPath)
	defer os.Unsetenv("GUR_DB_PATH")

	if _, err := db.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.CloseDB()

	// Create a closed task with verbose content
	task := &models.Task{
		ID:    "gur-compact-test",
		Title: "Test task for compaction",
		Description: "This is a very long description that should be cleared after compaction. " +
			"It contains detailed information about what needs to be done.",
		Notes:       "These are detailed notes that will also be cleared.",
		Status:      models.StatusClosed,
		CloseReason: "Completed successfully",
		Type:        models.TypeFeature,
		ClosedAt:    timePtr(time.Now()),
	}

	if err := db.GetDB().Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Compact the task
	task.Compact()
	if err := db.GetDB().Save(task).Error; err != nil {
		t.Fatalf("failed to save compacted task: %v", err)
	}

	// Retrieve and verify
	var retrieved models.Task
	if err := db.GetDB().First(&retrieved, "id = ?", "gur-compact-test").Error; err != nil {
		t.Fatalf("failed to retrieve task: %v", err)
	}

	if !retrieved.Compacted {
		t.Error("task should be marked as compacted")
	}
	if retrieved.Description != "" {
		t.Errorf("description should be empty after compaction, got %q", retrieved.Description)
	}
	if retrieved.Notes != "" {
		t.Errorf("notes should be empty after compaction, got %q", retrieved.Notes)
	}
	if retrieved.Summary == "" {
		t.Error("summary should be set after compaction")
	}
	// Summary should contain the title
	if retrieved.Summary != "[feature] Test task for compaction | Closed: Completed successfully" {
		t.Errorf("unexpected summary: %q", retrieved.Summary)
	}
}

func TestCompactPreservesEssentialFields(t *testing.T) {
	// Setup temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("GUR_DB_PATH", dbPath)
	defer os.Unsetenv("GUR_DB_PATH")

	if _, err := db.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.CloseDB()

	now := time.Now()
	task := &models.Task{
		ID:          "gur-preserve-test",
		Title:       "Important task",
		Status:      models.StatusClosed,
		Priority:    models.PriorityHigh,
		Type:        models.TypeBug,
		Assignee:    "developer",
		Labels:      models.StringSlice{"critical", "production"},
		CloseReason: "Fixed",
		ClosedAt:    &now,
	}

	if err := db.GetDB().Create(task).Error; err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	task.Compact()
	if err := db.GetDB().Save(task).Error; err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	var retrieved models.Task
	if err := db.GetDB().First(&retrieved, "id = ?", "gur-preserve-test").Error; err != nil {
		t.Fatalf("failed to retrieve: %v", err)
	}

	// These fields should be preserved
	if retrieved.Title != "Important task" {
		t.Errorf("title not preserved: %q", retrieved.Title)
	}
	if retrieved.Priority != models.PriorityHigh {
		t.Errorf("priority not preserved: %d", retrieved.Priority)
	}
	if retrieved.Type != models.TypeBug {
		t.Errorf("type not preserved: %q", retrieved.Type)
	}
	if retrieved.Assignee != "developer" {
		t.Errorf("assignee not preserved: %q", retrieved.Assignee)
	}
	if len(retrieved.Labels) != 2 {
		t.Errorf("labels not preserved: %v", retrieved.Labels)
	}
	if retrieved.CloseReason != "Fixed" {
		t.Errorf("close reason not preserved: %q", retrieved.CloseReason)
	}
}

func TestCompactAlreadyCompacted(t *testing.T) {
	task := &models.Task{
		ID:        "gur-already",
		Title:     "Already compacted",
		Summary:   "Existing summary",
		Compacted: true,
	}

	originalSummary := task.Summary
	task.Compact()

	// Should not change an already compacted task
	if task.Summary != originalSummary {
		t.Errorf("summary changed on already compacted task: %q -> %q", originalSummary, task.Summary)
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
