package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var reopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a closed task",
	Args:  cobra.ExactArgs(1),
	RunE:  runReopen,
}

func init() {
	rootCmd.AddCommand(reopenCmd)
}

func runReopen(cmd *cobra.Command, args []string) error {
	task, err := db.GetTaskByID(args[0])
	if err != nil {
		return fmt.Errorf("cannot reopen task: task '%s' not found (use 'gur list' to see available tasks)", args[0])
	}

	if !task.IsClosed() {
		return fmt.Errorf("cannot reopen task '%s': task is not closed (current status: %s)", task.ID, task.Status)
	}

	database := db.GetDB()
	models.RecordChange(database, task.ID, "status", task.Status, models.StatusOpen, "user")
	task.Reopen()
	if err := database.Save(&task).Error; err != nil {
		return fmt.Errorf("failed to reopen task '%s': database error: %w", task.ID, err)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "task": task})
	} else {
		fmt.Printf("Reopened: %s\n", task.ID)
	}
	return nil
}
