package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

func TestTemplateCreate(t *testing.T) {
	// Setup temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("GUR_DB_PATH", dbPath)
	defer os.Unsetenv("GUR_DB_PATH")

	if _, err := db.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.CloseDB()

	// Create a template
	template := &models.Template{
		Name:        "bug-report",
		Title:       "Bug: ",
		Description: "Steps to reproduce:\n1.\n2.\n3.",
		Priority:    models.PriorityHigh,
		Type:        models.TypeBug,
		Labels:      models.StringSlice{"bug", "needs-triage"},
	}

	if err := db.GetDB().Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Verify template was created with ID
	if template.ID == "" {
		t.Error("template ID should be set after creation")
	}

	// Retrieve template
	var retrieved models.Template
	if err := db.GetDB().Where("name = ?", "bug-report").First(&retrieved).Error; err != nil {
		t.Fatalf("failed to retrieve template: %v", err)
	}

	if retrieved.Name != "bug-report" {
		t.Errorf("name = %q, want %q", retrieved.Name, "bug-report")
	}
	if retrieved.Title != "Bug: " {
		t.Errorf("title = %q, want %q", retrieved.Title, "Bug: ")
	}
	if retrieved.Priority != models.PriorityHigh {
		t.Errorf("priority = %d, want %d", retrieved.Priority, models.PriorityHigh)
	}
	if retrieved.Type != models.TypeBug {
		t.Errorf("type = %q, want %q", retrieved.Type, models.TypeBug)
	}
}

func TestTemplateLabels(t *testing.T) {
	// Setup temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("GUR_DB_PATH", dbPath)
	defer os.Unsetenv("GUR_DB_PATH")

	if _, err := db.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.CloseDB()

	// Create template with labels
	template := &models.Template{
		Name:   "feature-request",
		Labels: models.StringSlice{"feature", "enhancement", "v2"},
	}

	if err := db.GetDB().Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Retrieve and verify labels
	var retrieved models.Template
	if err := db.GetDB().Where("name = ?", "feature-request").First(&retrieved).Error; err != nil {
		t.Fatalf("failed to retrieve template: %v", err)
	}

	if len(retrieved.Labels) != 3 {
		t.Errorf("labels count = %d, want 3", len(retrieved.Labels))
	}

	expectedLabels := []string{"feature", "enhancement", "v2"}
	for i, label := range expectedLabels {
		if i >= len(retrieved.Labels) || retrieved.Labels[i] != label {
			t.Errorf("label[%d] = %q, want %q", i, retrieved.Labels[i], label)
		}
	}
}

func TestTemplateDelete(t *testing.T) {
	// Setup temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("GUR_DB_PATH", dbPath)
	defer os.Unsetenv("GUR_DB_PATH")

	if _, err := db.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.CloseDB()

	// Create template
	template := &models.Template{
		Name: "to-delete",
	}
	if err := db.GetDB().Create(template).Error; err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	// Delete template
	result := db.GetDB().Where("name = ?", "to-delete").Delete(&models.Template{})
	if result.Error != nil {
		t.Fatalf("failed to delete template: %v", result.Error)
	}
	if result.RowsAffected != 1 {
		t.Errorf("rows affected = %d, want 1", result.RowsAffected)
	}

	// Verify deleted
	var count int64
	db.GetDB().Model(&models.Template{}).Where("name = ?", "to-delete").Count(&count)
	if count != 0 {
		t.Errorf("template still exists after deletion")
	}
}

func TestTemplateDuplicateName(t *testing.T) {
	// Setup temp database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	os.Setenv("GUR_DB_PATH", dbPath)
	defer os.Unsetenv("GUR_DB_PATH")

	if _, err := db.InitDB(dbPath); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.CloseDB()

	// Create first template
	template1 := &models.Template{Name: "unique-name"}
	if err := db.GetDB().Create(template1).Error; err != nil {
		t.Fatalf("failed to create first template: %v", err)
	}

	// Try to create duplicate - should fail due to unique constraint
	template2 := &models.Template{Name: "unique-name"}
	err := db.GetDB().Create(template2).Error
	if err == nil {
		t.Error("expected error when creating duplicate template name")
	}
}
