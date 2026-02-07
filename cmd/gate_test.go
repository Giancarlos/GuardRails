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
		ID:    "gate-testtest",
		Title: "Test Gate",
		Type:  "test",
	}
	if err := database.Create(gate).Error; err != nil {
		t.Fatalf("Failed to create gate: %v", err)
	}

	// Link gate to task with pending status (per-task verification)
	link := &models.GateTaskLink{
		GateID: gate.ID,
		TaskID: task.ID,
		Status: models.GateLinkPending,
	}
	if err := database.Create(link).Error; err != nil {
		t.Fatalf("Failed to create link: %v", err)
	}

	// Test 2: Link status pending - should fail
	err = CheckGatesBeforeClose(task.ID)
	if err == nil {
		t.Error("CheckGatesBeforeClose() with pending link should fail")
	}

	// Test 3: Link status failed - should fail
	link.Status = models.GateLinkFailed
	database.Save(link)

	err = CheckGatesBeforeClose(task.ID)
	if err == nil {
		t.Error("CheckGatesBeforeClose() with failed link should fail")
	}

	// Test 4: Link status passed - should pass
	link.Status = models.GateLinkPassed
	database.Save(link)

	err = CheckGatesBeforeClose(task.ID)
	if err != nil {
		t.Errorf("CheckGatesBeforeClose() with passed link should pass, got: %v", err)
	}

	// Test 5: Empty status (legacy) - blocks close
	link.Status = ""
	database.Save(link)

	err = CheckGatesBeforeClose(task.ID)
	if err == nil {
		t.Error("CheckGatesBeforeClose() with empty status should fail")
	}
}

func TestGetFailingGateLinksForTask(t *testing.T) {
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

	// Create gates and links with different per-task statuses
	gates := []*models.Gate{
		{ID: "gate-pass0001", Title: "Passed Gate"},
		{ID: "gate-fail0001", Title: "Failed Gate"},
		{ID: "gate-pend0001", Title: "Pending Gate"},
	}

	links := []*models.GateTaskLink{
		{GateID: "gate-pass0001", TaskID: task.ID, Status: models.GateLinkPassed},
		{GateID: "gate-fail0001", TaskID: task.ID, Status: models.GateLinkFailed},
		{GateID: "gate-pend0001", TaskID: task.ID, Status: models.GateLinkPending},
	}

	for _, g := range gates {
		database.Create(g)
	}
	for _, l := range links {
		database.Create(l)
	}

	// Get failing gate links (per-task status check)
	failing, err := GetFailingGateLinksForTask(task.ID)
	if err != nil {
		t.Fatalf("GetFailingGateLinksForTask() error: %v", err)
	}

	// Should have 2 failing (failed + pending - only passed allows close)
	if len(failing) != 2 {
		t.Errorf("GetFailingGateLinksForTask() returned %d, want 2", len(failing))
	}

	// Verify passed gate is NOT in failing list
	for _, info := range failing {
		if info.Status == models.GateLinkPassed {
			t.Error("GetFailingGateLinksForTask() should not include passed links")
		}
	}
}
