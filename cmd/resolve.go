package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Giancarlos/guardrails/internal/db"
	"github.com/Giancarlos/guardrails/internal/models"
)

// resolveTaskID is the standard entry point for every command that takes a single
// task ID arg. Returns a user-ready error message; callers can return it directly.
//
// All commands taking a <task-id> arg MUST use this instead of db.GetTaskByID so
// users get consistent prefix-matching and error wording across the tool.
func resolveTaskID(input string) (*models.Task, error) {
	task, err := db.ResolveTaskID(input)
	if err == nil {
		return task, nil
	}
	var ambig *db.AmbiguousIDError
	if errors.As(err, &ambig) {
		return nil, fmt.Errorf("%s\nUse more characters to disambiguate", ambig.Error())
	}
	if errors.Is(err, db.ErrTaskNotFound) {
		return nil, fmt.Errorf("task '%s' not found (use 'gur list' to see available tasks, or 'gur search' to find by keyword)", input)
	}
	// Unexpected DB error — propagate as-is.
	return nil, err
}

// resolveTaskIDs resolves a batch of task ID inputs with all-or-nothing semantics.
// If ANY input is ambiguous or not found, returns an aggregated error and zero tasks
// — never returns a partial list. This prevents partial-mutation footguns when a
// future command takes variadic IDs.
//
// Order of returned tasks matches input order on success.
func resolveTaskIDs(inputs []string) ([]*models.Task, error) {
	tasks := make([]*models.Task, 0, len(inputs))
	var problems []string
	for _, in := range inputs {
		task, err := db.ResolveTaskID(in)
		if err != nil {
			var ambig *db.AmbiguousIDError
			switch {
			case errors.As(err, &ambig):
				problems = append(problems, fmt.Sprintf("  %q: %s", in, ambig.Error()))
			case errors.Is(err, db.ErrTaskNotFound):
				problems = append(problems, fmt.Sprintf("  %q: not found", in))
			default:
				problems = append(problems, fmt.Sprintf("  %q: %v", in, err))
			}
			continue
		}
		tasks = append(tasks, task)
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("could not resolve %d of %d task IDs (no changes made):\n%s",
			len(problems), len(inputs), strings.Join(problems, "\n"))
	}
	return tasks, nil
}
