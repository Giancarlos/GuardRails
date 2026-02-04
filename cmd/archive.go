package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	archiveBefore string
	archiveAll    bool
)

var archiveCmd = &cobra.Command{
	Use:   "archive [task-id]",
	Short: "Archive closed tasks",
	Long: `Archive closed tasks to hide them from the default list view.

Examples:
  gur archive gur-abc123        # Archive a specific task
  gur archive --all             # Archive all closed tasks
  gur archive --before 30d      # Archive tasks closed more than 30 days ago
  gur archive --before 7d --all # Archive all tasks closed more than 7 days ago`,
	RunE: runArchive,
}

var unarchiveCmd = &cobra.Command{
	Use:   "unarchive <task-id>",
	Short: "Restore an archived task",
	Args:  cobra.ExactArgs(1),
	RunE:  runUnarchive,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(unarchiveCmd)
	archiveCmd.Flags().StringVar(&archiveBefore, "before", "", "Archive tasks closed before duration (e.g., 30d, 7d)")
	archiveCmd.Flags().BoolVar(&archiveAll, "all", false, "Archive all closed tasks (or all matching --before)")
}

func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}
	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]
	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", valueStr)
	}

	switch unit {
	case 'd':
		return time.Duration(value) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case 'h':
		return time.Duration(value) * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit: %c (use d=days, w=weeks, h=hours)", unit)
	}
}

func runArchive(cmd *cobra.Command, args []string) error {
	// Archive specific task
	if len(args) == 1 {
		taskID := args[0]
		var task models.Task
		if err := db.GetDB().First(&task, "id = ?", taskID).Error; err != nil {
			return fmt.Errorf("task not found: %s", taskID)
		}
		if task.Status != models.StatusClosed {
			return fmt.Errorf("only closed tasks can be archived (current status: %s)", task.Status)
		}
		task.Archive()
		if err := db.GetDB().Save(&task).Error; err != nil {
			return err
		}
		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{"archived": taskID})
			return nil
		}
		fmt.Printf("Archived: %s\n", taskID)
		return nil
	}

	// Bulk archive
	if !archiveAll && archiveBefore == "" {
		return fmt.Errorf("specify a task ID, --all, or --before")
	}

	query := db.GetDB().Model(&models.Task{}).Where("status = ?", models.StatusClosed)

	if archiveBefore != "" {
		duration, err := parseDuration(archiveBefore)
		if err != nil {
			return err
		}
		cutoff := time.Now().Add(-duration)
		query = query.Where("closed_at < ?", cutoff)
	}

	result := query.Update("status", models.StatusArchived)
	if result.Error != nil {
		return result.Error
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"archived_count": result.RowsAffected})
		return nil
	}
	fmt.Printf("Archived %d tasks\n", result.RowsAffected)
	return nil
}

func runUnarchive(cmd *cobra.Command, args []string) error {
	taskID := args[0]
	var task models.Task
	if err := db.GetDB().First(&task, "id = ?", taskID).Error; err != nil {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Status != models.StatusArchived {
		return fmt.Errorf("task is not archived (current status: %s)", task.Status)
	}
	task.Unarchive()
	if err := db.GetDB().Save(&task).Error; err != nil {
		return err
	}
	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"unarchived": taskID})
		return nil
	}
	fmt.Printf("Unarchived: %s (now closed)\n", taskID)
	return nil
}
