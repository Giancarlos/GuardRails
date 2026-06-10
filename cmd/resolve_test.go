package cmd

import (
	"strings"
	"testing"

	"github.com/Giancarlos/guardrails/internal/db"
	"github.com/Giancarlos/guardrails/internal/models"
)

// seedTask is a tiny helper to keep cmd-layer tests focused on the wrapper logic.
func seedTask(t *testing.T, id string) {
	t.Helper()
	task := &models.Task{
		ID:       id,
		Title:    "seed " + id,
		Status:   models.StatusOpen,
		Priority: models.PriorityMedium,
		Type:     models.TypeTask,
	}
	if err := db.GetDB().Create(task).Error; err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func TestResolveTaskID_FormatsAmbiguousError(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a1111111")
	seedTask(t, "gur-a2222222")

	_, err := resolveTaskID("a")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{`"a"`, "2 tasks", "gur-a1111111", "gur-a2222222", "Use more characters"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestResolveTaskID_FormatsNotFoundWithHint(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a60a05f4")

	_, err := resolveTaskID("gur-zzzzzzzz")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// Preserve the existing not-found wording so any consumers grepping the string keep working.
	for _, want := range []string{"not found", "gur list", "gur search"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q (regression — error wording must remain stable):\n%s", want, msg)
		}
	}
}

func TestResolveTaskID_HappyPath(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a60a05f4")

	got, err := resolveTaskID("a60a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "gur-a60a05f4" {
		t.Errorf("got %q, want gur-a60a05f4", got.ID)
	}
}
