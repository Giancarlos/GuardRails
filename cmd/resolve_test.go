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

func TestResolveTaskIDs_AllUnique(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a1111111")
	seedTask(t, "gur-b2222222")
	seedTask(t, "gur-c3333333")

	got, err := resolveTaskIDs([]string{"a1", "b2", "c3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d tasks, want 3", len(got))
	}
	// Order must match input order.
	wantOrder := []string{"gur-a1111111", "gur-b2222222", "gur-c3333333"}
	for i, w := range wantOrder {
		if got[i].ID != w {
			t.Errorf("[%d] got %q, want %q", i, got[i].ID, w)
		}
	}
}

func TestResolveTaskIDs_OneAmbiguousAbortsAll(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a1111111")
	seedTask(t, "gur-a2222222") // makes "a" ambiguous
	seedTask(t, "gur-b1234567")

	got, err := resolveTaskIDs([]string{"b1", "a"})
	if err == nil {
		t.Fatal("expected error")
	}
	if got != nil {
		t.Errorf("expected nil tasks on error (no partial list), got %d", len(got))
	}
	if !strings.Contains(err.Error(), "no changes made") {
		t.Errorf("error must explicitly state no changes made:\n%s", err.Error())
	}
	if !strings.Contains(err.Error(), `"a"`) {
		t.Errorf("error must name the ambiguous input:\n%s", err.Error())
	}
}

func TestResolveTaskIDs_NotFoundAbortsAll(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a1111111")

	got, err := resolveTaskIDs([]string{"a1", "zzzz"})
	if err == nil {
		t.Fatal("expected error")
	}
	if got != nil {
		t.Errorf("expected nil tasks on error, got %d", len(got))
	}
	if !strings.Contains(err.Error(), `"zzzz": not found`) {
		t.Errorf("error must name the missing input:\n%s", err.Error())
	}
}

func TestResolveTaskIDs_ReportsAllProblems(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a1111111")
	seedTask(t, "gur-a2222222") // makes "a" ambiguous

	_, err := resolveTaskIDs([]string{"a", "zzzz"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// Both problems must appear, not just the first.
	if !strings.Contains(msg, `"a"`) {
		t.Errorf("missing first problem in error:\n%s", msg)
	}
	if !strings.Contains(msg, `"zzzz"`) {
		t.Errorf("missing second problem in error:\n%s", msg)
	}
	if !strings.Contains(msg, "2 of 2") {
		t.Errorf("missing problem count summary:\n%s", msg)
	}
}

func TestResolveTaskIDs_DoesNotMutateDB(t *testing.T) {
	withTempDB(t)
	seedTask(t, "gur-a1111111")
	seedTask(t, "gur-a2222222") // ambiguous "a"

	var beforeCount int64
	db.GetDB().Model(&models.Task{}).Count(&beforeCount)

	_, _ = resolveTaskIDs([]string{"a", "zzzz"})

	var afterCount int64
	db.GetDB().Model(&models.Task{}).Count(&afterCount)

	if beforeCount != afterCount {
		t.Errorf("task count changed: before=%d after=%d (resolver must be read-only)",
			beforeCount, afterCount)
	}
}
