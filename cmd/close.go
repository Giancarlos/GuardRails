package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	closeReason string
	closeForce  bool
)

var closeCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runClose,
}

func init() {
	rootCmd.AddCommand(closeCmd)
	closeCmd.Flags().StringVarP(&closeReason, "reason", "r", "", "Reason for closing")
	closeCmd.Flags().BoolVarP(&closeForce, "force", "f", false, "Force close")
	closeCmd.MarkFlagRequired("reason")
}

func runClose(cmd *cobra.Command, args []string) error {
	database := db.GetDB()

	// First, find the task
	task, err := db.GetTaskByID(args[0])
	if err != nil {
		return fmt.Errorf("cannot close task: task '%s' not found (use 'gur list' to see available tasks)", args[0])
	}

	if task.IsClosed() {
		return fmt.Errorf("cannot close task '%s': already closed on %s with reason: %s",
			task.ID, task.ClosedAt.Format(models.DateTimeShortFormat), task.CloseReason)
	}

	if !closeForce {
		// Check for open blockers
		var blockerCount int64
		database.Model(&models.Dependency{}).
			Joins("JOIN tasks ON tasks.id = dependencies.parent_id").
			Where("dependencies.child_id = ? AND dependencies.type = ? AND tasks.status != ?",
				task.ID, models.DepTypeBlocks, models.StatusClosed).
			Count(&blockerCount)

		if blockerCount > 0 {
			return fmt.Errorf("cannot close task '%s': blocked by %d open task(s) (use 'gur show %s' to see blockers, or --force to override)",
				task.ID, blockerCount, task.ID)
		}

		// Check for open subtasks
		var openSubtasks int64
		database.Model(&models.Task{}).
			Where("parent_id = ? AND status != ?", task.ID, models.StatusClosed).
			Count(&openSubtasks)

		if openSubtasks > 0 {
			return fmt.Errorf("cannot close task '%s': has %d open subtask(s) (close subtasks first, or use --force to override)",
				task.ID, openSubtasks)
		}

		// Check for linked gates that haven't passed
		if err := CheckGatesBeforeClose(task.ID); err != nil {
			return err
		}
	}

	// Record history and close
	models.RecordChange(database, task.ID, "status", task.Status, models.StatusClosed, "user")
	models.RecordChange(database, task.ID, "close_reason", "", closeReason, "user")
	task.Close(closeReason)
	if err := database.Save(&task).Error; err != nil {
		return fmt.Errorf("failed to close task '%s': database error: %w", task.ID, err)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "task": task})
	} else {
		fmt.Printf("Closed: %s\n", task.ID)
	}
	return nil
}
