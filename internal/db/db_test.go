package db

import (
	"os"
	"path/filepath"
	"testing"

	"guardrails/internal/models"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "gur-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	_, err = InitDB(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to init test DB: %v", err)
	}

	// Return cleanup function
	return func() {
		CloseDB()
		os.RemoveAll(tmpDir)
	}
}

func TestInitDB(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db := GetDB()
	if db == nil {
		t.Fatal("GetDB() returned nil after InitDB")
	}
}

func TestGetTaskByID(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db := GetDB()

	// Create a test task
	task := &models.Task{
		ID:       "gur-testtest",
		Title:    "Test Task",
		Status:   models.StatusOpen,
		Priority: models.PriorityMedium,
		Type:     models.TypeTask,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("Failed to create test task: %v", err)
	}

	// Test GetTaskByID
	found, err := GetTaskByID("gur-testtest")
	if err != nil {
		t.Fatalf("GetTaskByID() error: %v", err)
	}
	if found.Title != "Test Task" {
		t.Errorf("GetTaskByID() title = %s, want Test Task", found.Title)
	}

	// Test not found
	_, err = GetTaskByID("gur-notfound")
	if err == nil {
		t.Error("GetTaskByID() should error for non-existent task")
	}
}

func TestGetGateByID(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	db := GetDB()

	// Create a test gate
	gate := &models.Gate{
		ID:    "gate-testtest",
		Title: "Test Gate",
		Type:  "test",
	}
	if err := db.Create(gate).Error; err != nil {
		t.Fatalf("Failed to create test gate: %v", err)
	}

	// Test GetGateByID
	found, err := GetGateByID("gate-testtest")
	if err != nil {
		t.Fatalf("GetGateByID() error: %v", err)
	}
	if found.Title != "Test Gate" {
		t.Errorf("GetGateByID() title = %s, want Test Gate", found.Title)
	}

	// Test not found
	_, err = GetGateByID("gate-notfound")
	if err == nil {
		t.Error("GetGateByID() should error for non-existent gate")
	}
}

func TestSetGetConfig(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Set config
	err := SetConfig("test_key", "test_value")
	if err != nil {
		t.Fatalf("SetConfig() error: %v", err)
	}

	// Get config
	value, err := GetConfig("test_key")
	if err != nil {
		t.Fatalf("GetConfig() error: %v", err)
	}
	if value != "test_value" {
		t.Errorf("GetConfig() = %s, want test_value", value)
	}

	// Update config
	err = SetConfig("test_key", "updated_value")
	if err != nil {
		t.Fatalf("SetConfig() update error: %v", err)
	}

	value, err = GetConfig("test_key")
	if err != nil {
		t.Fatalf("GetConfig() after update error: %v", err)
	}
	if value != "updated_value" {
		t.Errorf("GetConfig() after update = %s, want updated_value", value)
	}

	// Get non-existent config
	_, err = GetConfig("nonexistent")
	if err == nil {
		t.Error("GetConfig() should error for non-existent key")
	}
}

func TestCloseDB(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	err := CloseDB()
	if err != nil {
		t.Fatalf("CloseDB() error: %v", err)
	}

	// Should be nil after close
	if GetDB() != nil {
		t.Error("GetDB() should return nil after CloseDB()")
	}

	// Calling CloseDB again should be safe
	err = CloseDB()
	if err != nil {
		t.Errorf("CloseDB() second call error: %v", err)
	}
}
