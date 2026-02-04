package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	compactBefore  string
	compactAll     bool
	compactSummary bool
)

var compactCmd = &cobra.Command{
	Use:   "compact [task-id]",
	Short: "Compact closed tasks to save context space",
	Long: `Compact closed tasks by generating a summary and clearing verbose fields.

This is useful for AI agents to reduce context window usage while preserving
essential task information.

Examples:
  gur compact gur-abc123          # Compact a specific task
  gur compact --all               # Compact all closed tasks
  gur compact --before 7d         # Compact tasks closed more than 7 days ago
  gur compact --summary           # Show what would be compacted (dry run)`,
	RunE: runCompact,
}

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Generate a session summary of recent task activity",
	RunE:  runSummary,
}

func init() {
	rootCmd.AddCommand(compactCmd)
	rootCmd.AddCommand(summaryCmd)
	compactCmd.Flags().StringVar(&compactBefore, "before", "", "Compact tasks closed before duration (e.g., 7d, 30d)")
	compactCmd.Flags().BoolVar(&compactAll, "all", false, "Compact all closed tasks")
	compactCmd.Flags().BoolVar(&compactSummary, "dry-run", false, "Show what would be compacted without making changes")
}

func runCompact(cmd *cobra.Command, args []string) error {
	database := db.GetDB()

	// Compact specific task
	if len(args) == 1 {
		taskID := args[0]
		var task models.Task
		if err := database.First(&task, "id = ?", taskID).Error; err != nil {
			return fmt.Errorf("task not found: %s", taskID)
		}
		if task.Status != models.StatusClosed && task.Status != models.StatusArchived {
			return fmt.Errorf("only closed or archived tasks can be compacted (current status: %s)", task.Status)
		}
		if task.Compacted {
			return fmt.Errorf("task already compacted")
		}

		if compactSummary {
			fmt.Printf("Would compact: %s - %s\n", task.ID, task.Title)
			fmt.Printf("  Description length: %d chars\n", len(task.Description))
			fmt.Printf("  Notes length: %d chars\n", len(task.Notes))
			return nil
		}

		task.Compact()
		if err := database.Save(&task).Error; err != nil {
			return err
		}

		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{"compacted": taskID, "summary": task.Summary})
			return nil
		}
		fmt.Printf("Compacted: %s\n", taskID)
		fmt.Printf("Summary: %s\n", task.Summary)
		return nil
	}

	// Bulk compact
	if !compactAll && compactBefore == "" {
		return fmt.Errorf("specify a task ID, --all, or --before")
	}

	query := database.Model(&models.Task{}).
		Where("status IN ?", []string{models.StatusClosed, models.StatusArchived}).
		Where("compacted = ?", false)

	if compactBefore != "" {
		duration, err := parseDuration(compactBefore)
		if err != nil {
			return err
		}
		cutoff := time.Now().Add(-duration)
		query = query.Where("closed_at < ?", cutoff)
	}

	// Get tasks to compact
	var tasks []models.Task
	if err := query.Find(&tasks).Error; err != nil {
		return err
	}

	if compactSummary {
		if len(tasks) == 0 {
			fmt.Println("No tasks to compact")
			return nil
		}
		totalDescLen := 0
		totalNotesLen := 0
		for _, t := range tasks {
			totalDescLen += len(t.Description)
			totalNotesLen += len(t.Notes)
			fmt.Printf("Would compact: %s - %s\n", t.ID, t.Title)
		}
		fmt.Printf("\nTotal: %d tasks, %d chars in descriptions, %d chars in notes\n",
			len(tasks), totalDescLen, totalNotesLen)
		return nil
	}

	// Compact each task
	compactedCount := 0
	for _, task := range tasks {
		task.Compact()
		if err := database.Save(&task).Error; err != nil {
			return err
		}
		compactedCount++
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"compacted_count": compactedCount})
		return nil
	}
	fmt.Printf("Compacted %d tasks\n", compactedCount)
	return nil
}

func runSummary(cmd *cobra.Command, args []string) error {
	database := db.GetDB()

	// Get counts by status
	var openCount, inProgressCount, closedCount, archivedCount int64
	database.Model(&models.Task{}).Where("status = ?", models.StatusOpen).Count(&openCount)
	database.Model(&models.Task{}).Where("status = ?", models.StatusInProgress).Count(&inProgressCount)
	database.Model(&models.Task{}).Where("status = ?", models.StatusClosed).Count(&closedCount)
	database.Model(&models.Task{}).Where("status = ?", models.StatusArchived).Count(&archivedCount)

	// Get recent activity (last 24 hours)
	yesterday := time.Now().Add(-24 * time.Hour)
	var recentlyCreated, recentlyClosed int64
	database.Model(&models.Task{}).Where("created_at > ?", yesterday).Count(&recentlyCreated)
	database.Model(&models.Task{}).Where("closed_at > ?", yesterday).Count(&recentlyClosed)

	// Get high priority open tasks
	var highPriorityTasks []models.Task
	database.Where("status IN ? AND priority <= 1", []string{models.StatusOpen, models.StatusInProgress}).
		Order("priority ASC, created_at ASC").
		Limit(5).
		Find(&highPriorityTasks)

	// Get compacted vs uncompacted
	var compactedCount, uncompactedCount int64
	database.Model(&models.Task{}).Where("compacted = ?", true).Count(&compactedCount)
	database.Model(&models.Task{}).Where("compacted = ? AND status IN ?", false, []string{models.StatusClosed, models.StatusArchived}).Count(&uncompactedCount)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"status_counts": map[string]int64{
				"open":        openCount,
				"in_progress": inProgressCount,
				"closed":      closedCount,
				"archived":    archivedCount,
			},
			"recent_24h": map[string]int64{
				"created": recentlyCreated,
				"closed":  recentlyClosed,
			},
			"high_priority_tasks": highPriorityTasks,
			"compaction": map[string]int64{
				"compacted":   compactedCount,
				"uncompacted": uncompactedCount,
			},
		})
		return nil
	}

	fmt.Println("=== Session Summary ===")
	fmt.Printf("Task Status:\n")
	fmt.Printf("  Open:        %d\n", openCount)
	fmt.Printf("  In Progress: %d\n", inProgressCount)
	fmt.Printf("  Closed:      %d\n", closedCount)
	fmt.Printf("  Archived:    %d\n", archivedCount)

	fmt.Printf("\nLast 24 Hours:\n")
	fmt.Printf("  Created: %d\n", recentlyCreated)
	fmt.Printf("  Closed:  %d\n", recentlyClosed)

	if len(highPriorityTasks) > 0 {
		fmt.Printf("\nHigh Priority Tasks:\n")
		for _, t := range highPriorityTasks {
			fmt.Printf("  [%s] P%d %s - %s\n", t.ID, t.Priority, t.Status, t.Title)
		}
	}

	fmt.Printf("\nMemory:\n")
	fmt.Printf("  Compacted:   %d tasks\n", compactedCount)
	fmt.Printf("  Uncompacted: %d tasks (run 'gur compact --all' to free space)\n", uncompactedCount)

	return nil
}
