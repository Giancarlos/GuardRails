package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	listStatus   string
	listPriority int
	listType     string
	listAssignee string
	listArchived bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Short:   "List tasks",
	Aliases: []string{"ls"},
	RunE:    runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter by status")
	listCmd.Flags().IntVarP(&listPriority, "priority", "p", -1, "Filter by priority")
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by type")
	listCmd.Flags().StringVarP(&listAssignee, "assignee", "a", "", "Filter by assignee")
	listCmd.Flags().BoolVar(&listArchived, "archived", false, "Include archived tasks")
}

func runList(cmd *cobra.Command, args []string) error {
	var tasks []models.Task
	query := db.GetDB().Order("priority ASC, created_at DESC")

	// Exclude archived by default unless --archived flag or filtering by archived status
	if !listArchived && listStatus != models.StatusArchived {
		query = query.Where("status != ?", models.StatusArchived)
	}

	if listStatus != "" {
		query = query.Where("status = ?", listStatus)
	}
	if listPriority >= 0 {
		query = query.Where("priority = ?", listPriority)
	}
	if listType != "" {
		query = query.Where("type = ?", listType)
	}
	if listAssignee != "" {
		query = query.Where("assignee = ?", listAssignee)
	}

	if err := query.Find(&tasks).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"count": len(tasks), "tasks": tasks})
		return nil
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return nil
	}

	for _, t := range tasks {
		indent := ""
		depth := models.GetDepth(t.ID)
		for i := 0; i < depth; i++ {
			indent += "  "
		}
		fmt.Printf("%s[%s] P%d %s - %s (%s)\n", indent, t.ID, t.Priority, t.Status, t.Title, t.Type)
	}
	return nil
}
