package db

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Giancarlos/guardrails/internal/models"
)

// ErrTaskNotFound is returned by ResolveTaskID when no task matches the input.
// Callers should branch on this with errors.Is to format their own user-facing message.
var ErrTaskNotFound = errors.New("task not found")

// AmbiguousIDError is returned when the input prefix matches more than one task.
// Matches is sorted ascending so callers get deterministic output.
type AmbiguousIDError struct {
	Input   string   // The original user input (not the internal query prefix).
	Matches []string // Sorted ascending. Capped at maxAmbiguousIDs.
}

func (e *AmbiguousIDError) Error() string {
	return fmt.Sprintf("ambiguous ID %q matches %d tasks: [%s]",
		e.Input, len(e.Matches), strings.Join(e.Matches, " "))
}

// maxAmbiguousIDs caps the number of IDs listed in an AmbiguousIDError, so the
// error message stays useful rather than dumping thousands of rows.
const maxAmbiguousIDs = 20

// ResolveTaskID looks up a task by an exact ID or unique prefix.
//
// Strategy order:
//  1. Exact match on the full ID.
//  2. If input does not already start with the IDPrefix ("gur-"), try IDPrefix+input
//     as a prefix. Lets users type just the hash chars (e.g. "a60a" → "gur-a60a*").
//  3. Bare prefix match on input as-is.
//
// Returns ErrTaskNotFound for empty/whitespace input or when nothing matches.
// Returns *AmbiguousIDError (with the user's original input echoed) when more than
// one task matches at any winning strategy.
func ResolveTaskID(input string) (*models.Task, error) {
	original := input
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, ErrTaskNotFound
	}

	// Strategy 1: exact match.
	if t, err := GetTaskByID(input); err == nil {
		return t, nil
	}

	// Strategy 2: auto-prepend IDPrefix if missing.
	if !strings.HasPrefix(input, models.IDPrefix) {
		matches, err := matchByPrefix(models.IDPrefix + input)
		if err != nil {
			return nil, err
		}
		if len(matches) == 1 {
			return &matches[0], nil
		}
		if len(matches) > 1 {
			return nil, ambiguousErr(original, matches)
		}
	}

	// Strategy 3: bare prefix.
	matches, err := matchByPrefix(input)
	if err != nil {
		return nil, err
	}
	switch len(matches) {
	case 0:
		return nil, ErrTaskNotFound
	case 1:
		return &matches[0], nil
	default:
		return nil, ambiguousErr(original, matches)
	}
}

// matchByPrefix returns up to maxAmbiguousIDs+1 tasks whose id starts with prefix.
// Returning one extra above the cap lets callers detect "more than the cap" without
// loading the world.
func matchByPrefix(prefix string) ([]models.Task, error) {
	var tasks []models.Task
	pattern := escapeLike(prefix) + "%"
	err := GetDB().Where("id LIKE ? ESCAPE '\\'", pattern).
		Order("id ASC").
		Limit(maxAmbiguousIDs + 1).
		Find(&tasks).Error
	return tasks, err
}

func ambiguousErr(input string, tasks []models.Task) *AmbiguousIDError {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	sort.Strings(ids)
	return &AmbiguousIDError{Input: input, Matches: ids}
}

// escapeLike escapes SQL LIKE wildcards (%, _) and the escape char itself.
// Required because user input could theoretically contain these.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
