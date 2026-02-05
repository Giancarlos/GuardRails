package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var historyLimit int

var historyCmd = &cobra.Command{
	Use:   "history <task-id>",
	Short: "Show change history for a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.Flags().IntVarP(&historyLimit, "limit", "n", 50, "Maximum entries to show")
}

func runHistory(cmd *cobra.Command, args []string) error {
	taskID := args[0]

	// Verify task exists
	task, err := db.GetTaskByID(taskID)
	if err != nil {
		return fmt.Errorf("task not found: %s", taskID)
	}

	var history []models.TaskHistory
	if err := db.GetDB().Where("task_id = ?", taskID).
		Order("changed_at DESC").
		Limit(historyLimit).
		Find(&history).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"task_id": taskID,
			"count":   len(history),
			"history": history,
		})
		return nil
	}

	if len(history) == 0 {
		fmt.Printf("No change history for task %s\n", taskID)
		return nil
	}

	fmt.Printf("Change history for %s (%s):\n\n", taskID, task.Title)
	for _, h := range history {
		timestamp := h.ChangedAt.Format(models.DateTimeFormat)
		if h.OldValue == "" {
			fmt.Printf("[%s] %s: set to \"%s\"", timestamp, h.Field, h.NewValue)
		} else if h.NewValue == "" {
			fmt.Printf("[%s] %s: removed \"%s\"", timestamp, h.Field, h.OldValue)
		} else {
			fmt.Printf("[%s] %s: \"%s\" → \"%s\"", timestamp, h.Field, h.OldValue, h.NewValue)
		}
		if h.ChangedBy != "" {
			fmt.Printf(" (by %s)", h.ChangedBy)
		}
		fmt.Println()
	}
	return nil
}
