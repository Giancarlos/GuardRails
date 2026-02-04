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
	var task models.Task
	if err := db.GetDB().Where("id = ?", args[0]).First(&task).Error; err != nil {
		return fmt.Errorf("task not found: %s", args[0])
	}

	if !task.IsClosed() {
		return fmt.Errorf("task is not closed")
	}

	database := db.GetDB()
	models.RecordChange(database, task.ID, "status", task.Status, models.StatusOpen, "user")
	task.Reopen()
	if err := database.Save(&task).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "task": task})
	} else {
		fmt.Printf("Reopened: %s\n", task.ID)
	}
	return nil
}
