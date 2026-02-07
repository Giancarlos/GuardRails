package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gur-cmd-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	_, err = db.InitDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init test DB: %v", err)
	}

	return func() {
		db.CloseDB()
		os.RemoveAll(tmpDir)
	}
}

func TestCheckGatesBeforeClose(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	database := db.GetDB()

	// Create a task
	task := &models.Task{
		ID:     "gur-testgate",
		Title:  "Test Task",
		Status: models.StatusOpen,
	}
	if err := database.Create(task).Error; err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Test 1: No gates linked - should FAIL (gates are required)
	err := CheckGatesBeforeClose(task.ID)
	if err == nil {
		t.Error("CheckGatesBeforeClose() with no gates should fail - gates are required")
	}

	// Create a gate
	gate := &models.Gate{
		ID:         "gate-testtest",
		Title:      "Test Gate",
		Type:       "test",
		LastResult: models.GatePending,
	}
	if err := database.Create(gate).Error; err != nil {
		t.Fatalf("Failed to create gate: %v", err)
	}

	// Link gate to task
	link := &models.GateTaskLink{
		GateID: gate.ID,
		TaskID: task.ID,
	}
	if err := database.Create(link).Error; err != nil {
		t.Fatalf("Failed to create link: %v", err)
	}

	// Test 2: Gate pending - should fail
	err = CheckGatesBeforeClose(task.ID)
	if err == nil {
		t.Error("CheckGatesBeforeClose() with pending gate should fail")
	}

	// Test 3: Gate failed - should fail
	gate.LastResult = models.GateFailed
	database.Save(gate)

	err = CheckGatesBeforeClose(task.ID)
	if err == nil {
		t.Error("CheckGatesBeforeClose() with failed gate should fail")
	}

	// Test 4: Gate passed - should pass
	gate.LastResult = models.GatePassed
	database.Save(gate)

	err = CheckGatesBeforeClose(task.ID)
	if err != nil {
		t.Errorf("CheckGatesBeforeClose() with passed gate should pass, got: %v", err)
	}

	// Test 5: Gate skipped - blocks close (only passed gates allow close)
	gate.LastResult = models.GateSkipped
	database.Save(gate)

	err = CheckGatesBeforeClose(task.ID)
	if err == nil {
		t.Error("CheckGatesBeforeClose() with skipped gate should fail (only passed gates allow close)")
	}
}

func TestGetFailingGatesForTask(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	database := db.GetDB()

	// Create a task
	task := &models.Task{
		ID:     "gur-failtest",
		Title:  "Test Task",
		Status: models.StatusOpen,
	}
	database.Create(task)

	// Create gates with different statuses
	gates := []*models.Gate{
		{ID: "gate-pass0001", Title: "Passed Gate", LastResult: models.GatePassed},
		{ID: "gate-fail0001", Title: "Failed Gate", LastResult: models.GateFailed},
		{ID: "gate-pend0001", Title: "Pending Gate", LastResult: models.GatePending},
		{ID: "gate-skip0001", Title: "Skipped Gate", LastResult: models.GateSkipped},
	}

	for _, g := range gates {
		database.Create(g)
		database.Create(&models.GateTaskLink{GateID: g.ID, TaskID: task.ID})
	}

	// Get failing gates
	failing, err := GetFailingGatesForTask(task.ID)
	if err != nil {
		t.Fatalf("GetFailingGatesForTask() error: %v", err)
	}

	// Should have 3 failing gates (failed, pending, skipped - only passed allows close)
	if len(failing) != 3 {
		t.Errorf("GetFailingGatesForTask() returned %d gates, want 3", len(failing))
	}

	// Verify passed gate is NOT in failing list
	for _, g := range failing {
		if g.LastResult == models.GatePassed {
			t.Error("GetFailingGatesForTask() should not include passed gates")
		}
	}
}
