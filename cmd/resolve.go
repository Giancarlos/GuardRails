package cmd

import (
	"errors"
	"fmt"

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
