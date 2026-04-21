package db

import (
	"errors"
	"testing"

	"github.com/Giancarlos/guardrails/internal/models"
)

// seedTasks creates tasks with known IDs for resolver testing.
func seedTasks(t *testing.T, ids ...string) {
	t.Helper()
	db := GetDB()
	for _, id := range ids {
		task := &models.Task{
			ID:       id,
			Title:    "seed " + id,
			Status:   models.StatusOpen,
			Priority: models.PriorityMedium,
			Type:     models.TypeTask,
		}
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
}

func TestResolveTaskID_ExactMatch(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a60a05f4", "gur-b1234567")

	got, err := ResolveTaskID("gur-a60a05f4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "gur-a60a05f4" {
		t.Errorf("got %q, want gur-a60a05f4", got.ID)
	}
}

func TestResolveTaskID_BareHashPrefix(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a60a05f4", "gur-b1234567")

	// "a60a" should resolve via strategy 2 (auto-prepend gur-).
	got, err := ResolveTaskID("a60a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "gur-a60a05f4" {
		t.Errorf("got %q, want gur-a60a05f4", got.ID)
	}
}

func TestResolveTaskID_PrefixedInput(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a60a05f4", "gur-b1234567")

	got, err := ResolveTaskID("gur-a60a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "gur-a60a05f4" {
		t.Errorf("got %q, want gur-a60a05f4", got.ID)
	}
}

func TestResolveTaskID_Ambiguous(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a1111111", "gur-a2222222", "gur-a3333333", "gur-b1234567")

	_, err := ResolveTaskID("a")
	if err == nil {
		t.Fatal("expected ambiguous error")
	}
	var ambig *AmbiguousIDError
	if !errors.As(err, &ambig) {
		t.Fatalf("got %T, want *AmbiguousIDError", err)
	}
	if ambig.Input != "a" {
		t.Errorf("Input = %q, want original user input %q", ambig.Input, "a")
	}
	wantIDs := []string{"gur-a1111111", "gur-a2222222", "gur-a3333333"}
	if len(ambig.Matches) != len(wantIDs) {
		t.Fatalf("Matches len = %d, want %d (%v)", len(ambig.Matches), len(wantIDs), ambig.Matches)
	}
	// Verify sorted ascending.
	for i, want := range wantIDs {
		if ambig.Matches[i] != want {
			t.Errorf("Matches[%d] = %q, want %q (full list %v)", i, ambig.Matches[i], want, ambig.Matches)
		}
	}
}

func TestResolveTaskID_AmbiguousErrorMessage(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a1111111", "gur-a2222222")

	_, err := ResolveTaskID("a")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{`"a"`, "2 tasks", "gur-a1111111", "gur-a2222222"} {
		if !contains(msg, want) {
			t.Errorf("error missing %q:\n%s", want, msg)
		}
	}
}

func TestResolveTaskID_NotFound(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a60a05f4")

	_, err := ResolveTaskID("gur-zzzzzzzz")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("got %v, want ErrTaskNotFound", err)
	}
}

func TestResolveTaskID_EmptyInput(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a60a05f4", "gur-b1234567")

	for _, input := range []string{"", "   ", "\t", "\n"} {
		_, err := ResolveTaskID(input)
		if !errors.Is(err, ErrTaskNotFound) {
			t.Errorf("input %q: got %v, want ErrTaskNotFound (no LIKE %% blowup)", input, err)
		}
	}
}

func TestResolveTaskID_PrefixOnly(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a1111111", "gur-a2222222", "gur-b1234567")

	// "gur-" alone should be ambiguous (matches all).
	_, err := ResolveTaskID("gur-")
	var ambig *AmbiguousIDError
	if !errors.As(err, &ambig) {
		t.Fatalf("got %v, want *AmbiguousIDError", err)
	}
	if len(ambig.Matches) != 3 {
		t.Errorf("Matches len = %d, want 3", len(ambig.Matches))
	}
}

func TestResolveTaskID_Deterministic(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a3333333", "gur-a1111111", "gur-a2222222")

	// Call multiple times; Matches slice must be identical every call.
	var first []string
	for i := range 5 {
		_, err := ResolveTaskID("a")
		var ambig *AmbiguousIDError
		if !errors.As(err, &ambig) {
			t.Fatalf("iter %d: not ambiguous", i)
		}
		if i == 0 {
			first = ambig.Matches
			continue
		}
		if len(ambig.Matches) != len(first) {
			t.Fatalf("iter %d: len mismatch %v vs %v", i, ambig.Matches, first)
		}
		for j := range first {
			if ambig.Matches[j] != first[j] {
				t.Fatalf("iter %d: Matches[%d] = %q, want %q", i, j, ambig.Matches[j], first[j])
			}
		}
	}
}

func TestResolveTaskID_LikeWildcardsEscaped(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a1111111")

	// Input with a literal % must not act as a wildcard (no task has % in its id).
	_, err := ResolveTaskID("%")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("input %q: got %v, want ErrTaskNotFound (wildcard must be escaped)", "%", err)
	}
	_, err = ResolveTaskID("_")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("input %q: got %v, want ErrTaskNotFound (wildcard must be escaped)", "_", err)
	}
}

func TestResolveTaskID_FullIDWithoutPrefix(t *testing.T) {
	defer setupTestDB(t)()
	seedTasks(t, "gur-a60a05f4")

	// "a60a05f4" — full hash minus prefix — should resolve uniquely via strategy 2.
	got, err := ResolveTaskID("a60a05f4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "gur-a60a05f4" {
		t.Errorf("got %q, want gur-a60a05f4", got.ID)
	}
}

// contains is a tiny helper to keep tests readable (strings.Contains inline gets noisy).
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
