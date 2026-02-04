package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show task details",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	database := db.GetDB()
	var task models.Task
	if err := database.Where("id = ?", args[0]).First(&task).Error; err != nil {
		return fmt.Errorf("task not found: %s", args[0])
	}

	// Use eager loading to fetch dependencies in fewer queries
	var blockedBy, blocks []models.Dependency
	database.Where("child_id = ?", task.ID).Find(&blockedBy)
	database.Where("parent_id = ?", task.ID).Find(&blocks)

	// Fetch subtasks
	var subtasks []models.Task
	database.Where("parent_id = ?", task.ID).Order("id ASC").Find(&subtasks)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"task": task, "blocked_by": blockedBy, "blocks": blocks, "subtasks": subtasks})
		return nil
	}

	fmt.Printf("ID:       %s\n", task.ID)
	if task.ParentID != "" {
		fmt.Printf("Parent:   %s\n", task.ParentID)
	}
	fmt.Printf("Title:    %s\n", task.Title)
	fmt.Printf("Status:   %s\n", task.Status)
	fmt.Printf("Priority: %s\n", task.PriorityString())
	fmt.Printf("Type:     %s\n", task.Type)
	if task.Description != "" {
		fmt.Printf("Desc:     %s\n", task.Description)
	}
	if task.Assignee != "" {
		fmt.Printf("Assignee: %s\n", task.Assignee)
	}
	if len(task.Labels) > 0 {
		fmt.Printf("Labels:   %v\n", task.Labels)
	}
	if task.Summary != "" {
		fmt.Printf("Summary:  %s\n", task.Summary)
	}
	fmt.Printf("Created:  %s\n", task.CreatedAt.Format(models.DateTimeShortFormat))
	if len(subtasks) > 0 {
		fmt.Println("\nSubtasks:")
		for _, s := range subtasks {
			fmt.Printf("  [%s] %s - %s\n", s.ID, s.Status, s.Title)
		}
	}
	if len(blockedBy) > 0 {
		fmt.Println("\nBlocked by:")
		for _, d := range blockedBy {
			fmt.Printf("  - %s\n", d.ParentID)
		}
	}
	if len(blocks) > 0 {
		fmt.Println("\nBlocks:")
		for _, d := range blocks {
			fmt.Printf("  - %s\n", d.ChildID)
		}
	}
	if task.Notes != "" {
		fmt.Printf("\nNotes:\n%s", task.Notes)
	}
	return nil
}
