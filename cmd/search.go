package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search tasks",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := "%" + strings.ToLower(args[0]) + "%"

	// Use database-side filtering with LIKE for better performance
	var matches []models.Task
	if err := db.GetDB().
		Where("LOWER(title) LIKE ? OR LOWER(description) LIKE ?", query, query).
		Order("priority ASC, created_at DESC").
		Find(&matches).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"count": len(matches), "tasks": matches})
		return nil
	}

	if len(matches) == 0 {
		fmt.Println("No matches found")
		return nil
	}

	for _, t := range matches {
		fmt.Printf("[%s] P%d %s - %s\n", t.ID, t.Priority, t.Status, t.Title)
	}
	return nil
}
