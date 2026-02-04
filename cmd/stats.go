package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show project statistics",
	RunE:  runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	database := db.GetDB()

	// Get status counts in a single query
	type statusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []statusCount
	database.Model(&models.Task{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&statusCounts)

	// Get priority counts in a single query
	type priorityCount struct {
		Priority int
		Count    int64
	}
	var priorityCounts []priorityCount
	database.Model(&models.Task{}).
		Select("priority, count(*) as count").
		Group("priority").
		Scan(&priorityCounts)

	// Build status map
	var total, open, inProgress, closed int64
	for _, sc := range statusCounts {
		total += sc.Count
		switch sc.Status {
		case models.StatusOpen:
			open = sc.Count
		case models.StatusInProgress:
			inProgress = sc.Count
		case models.StatusClosed:
			closed = sc.Count
		}
	}

	// Build priority map
	var p0, p1, p2, p3, p4 int64
	for _, pc := range priorityCounts {
		switch pc.Priority {
		case 0:
			p0 = pc.Count
		case 1:
			p1 = pc.Count
		case 2:
			p2 = pc.Count
		case 3:
			p3 = pc.Count
		case 4:
			p4 = pc.Count
		}
	}

	stats := map[string]interface{}{
		"total":       total,
		"open":        open,
		"in_progress": inProgress,
		"closed":      closed,
		"by_priority": map[string]int64{"p0": p0, "p1": p1, "p2": p2, "p3": p3, "p4": p4},
	}

	if IsJSONOutput() {
		OutputJSON(stats)
		return nil
	}

	fmt.Printf("Total tasks: %d\n\n", total)
	fmt.Println("By status:")
	fmt.Printf("  Open:        %d\n", open)
	fmt.Printf("  In Progress: %d\n", inProgress)
	fmt.Printf("  Closed:      %d\n", closed)
	fmt.Println("\nBy priority:")
	fmt.Printf("  P0: %d  P1: %d  P2: %d  P3: %d  P4: %d\n", p0, p1, p2, p3, p4)
	return nil
}
