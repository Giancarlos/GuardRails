package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "List tasks with no open blockers",
	RunE:  runReady,
}

func init() {
	rootCmd.AddCommand(readyCmd)
}

func runReady(cmd *cobra.Command, args []string) error {
	database := db.GetDB()

	// Get IDs of tasks that have open blockers (single query)
	var blockedTaskIDs []string
	database.Model(&models.Dependency{}).
		Select("DISTINCT dependencies.child_id").
		Joins("JOIN tasks ON tasks.id = dependencies.parent_id").
		Where("dependencies.type = ? AND tasks.status != ?",
			models.DepTypeBlocks, models.StatusClosed).
		Pluck("child_id", &blockedTaskIDs)

	// Get all open/in-progress tasks that are NOT in the blocked list (single query)
	var readyTasks []models.Task
	query := database.Where("status IN ?", []string{models.StatusOpen, models.StatusInProgress})
	if len(blockedTaskIDs) > 0 {
		query = query.Where("id NOT IN ?", blockedTaskIDs)
	}
	if err := query.Order("priority ASC, created_at DESC").Find(&readyTasks).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"count": len(readyTasks), "tasks": readyTasks})
		return nil
	}

	if len(readyTasks) == 0 {
		fmt.Println("No ready tasks")
		return nil
	}

	fmt.Printf("Ready tasks (%d):\n", len(readyTasks))
	for _, t := range readyTasks {
		fmt.Printf("[%s] P%d %s - %s\n", t.ID, t.Priority, t.Status, t.Title)
	}
	return nil
}
