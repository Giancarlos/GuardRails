package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Giancarlos/guardrails/internal/db"
	"github.com/Giancarlos/guardrails/internal/models"
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

func TestConfigShowHasThreeSections(t *testing.T) {
	tmpDir := withTempDB(t)
	withVerboseOutput(t)

	out := captureStdout(t, func() {
		if err := runConfigShow(nil, nil); err != nil {
			t.Fatalf("runConfigShow: %v", err)
		}
	})

	for _, want := range []string{"Configuration:", "GitHub:", "Paths:"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing section %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "Project Root: "+tmpDir) {
		t.Errorf("output missing 'Project Root: %s':\n%s", tmpDir, out)
	}
	// Database should now appear under Paths, not above the GitHub block.
	pathsIdx := strings.Index(out, "Paths:")
	dbIdx := strings.Index(out, "Database:")
	if pathsIdx == -1 || dbIdx == -1 || dbIdx < pathsIdx {
		t.Errorf("expected 'Database:' to appear after 'Paths:' header:\n%s", out)
	}
}

func TestConfigShowJSONIncludesProjectRoot(t *testing.T) {
	tmpDir := withTempDB(t)
	prevVerbose, prevJSON := verboseOutput, jsonOutput
	verboseOutput = false
	jsonOutput = true
	t.Cleanup(func() { verboseOutput = prevVerbose; jsonOutput = prevJSON })

	out := captureStdout(t, func() {
		if err := runConfigShow(nil, nil); err != nil {
			t.Fatalf("runConfigShow: %v", err)
		}
	})

	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if got := parsed["project_root"]; got != tmpDir {
		t.Errorf("project_root = %v, want %q", got, tmpDir)
	}
	// Existing flat keys should remain for backward compatibility.
	for _, k := range []string{"database", "mode", "schema_version", "github"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("expected flat JSON key %q to still be present", k)
		}
	}
}