package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

func TestConfigMachineSettings(t *testing.T) {
	// Setup temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("GUR_DB_PATH", dbPath)
	defer os.Unsetenv("GUR_DB_PATH")

	if _, err := db.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.CloseDB()

	// Test setting machine name
	if err := db.SetConfig(models.ConfigMachineName, "Work MacBook"); err != nil {
		t.Fatalf("failed to set machine name: %v", err)
	}

	name, err := db.GetConfig(models.ConfigMachineName)
	if err != nil {
		t.Fatalf("failed to get machine name: %v", err)
	}
	if name != "Work MacBook" {
		t.Errorf("machine name = %q, want %q", name, "Work MacBook")
	}

	// Test setting share preference
	if err := db.SetConfig(models.ConfigMachineShare, "true"); err != nil {
		t.Fatalf("failed to set share pref: %v", err)
	}

	share, err := db.GetConfig(models.ConfigMachineShare)
	if err != nil {
		t.Fatalf("failed to get share pref: %v", err)
	}
	if share != "true" {
		t.Errorf("share pref = %q, want %q", share, "true")
	}

	// Test updating share preference
	if err := db.SetConfig(models.ConfigMachineShare, "false"); err != nil {
		t.Fatalf("failed to update share pref: %v", err)
	}

	share, err = db.GetConfig(models.ConfigMachineShare)
	if err != nil {
		t.Fatalf("failed to get updated share pref: %v", err)
	}
	if share != "false" {
		t.Errorf("updated share pref = %q, want %q", share, "false")
	}
}

func TestConfigMachineNameConstants(t *testing.T) {
	// Verify constants are defined correctly
	if models.ConfigMachineName == "" {
		t.Error("ConfigMachineName constant is empty")
	}
	if models.ConfigMachineShare == "" {
		t.Error("ConfigMachineShare constant is empty")
	}

	// They should be different
	if models.ConfigMachineName == models.ConfigMachineShare {
		t.Error("ConfigMachineName and ConfigMachineShare should be different")
	}
}
