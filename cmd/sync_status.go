package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status with GitHub",
	Long:  `Show the current sync status between local gur database and GitHub.`,
	RunE:  runSyncStatus,
}

func init() {
	syncCmd.AddCommand(syncStatusCmd)
}

func runSyncStatus(cmd *cobra.Command, args []string) error {
	// Get GitHub configuration
	repo, err := db.GetConfig(models.ConfigGitHubRepo)
	if err != nil || repo == "" {
		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{
				"configured": false,
				"message":    "GitHub not configured",
			})
		} else {
			fmt.Println("GitHub not configured. Run 'gur config github' first.")
		}
		return nil
	}

	database := db.GetDB()

	// Count tasks by sync status
	var totalTasks int64
	var syncedTasks int64
	var unsyncedTasks int64
	var localTasks int64
	var githubTasks int64

	database.Model(&models.Task{}).Where("status != ?", models.StatusArchived).Count(&totalTasks)
	database.Model(&models.Task{}).Where("synced = ? AND status != ?", true, models.StatusArchived).Count(&syncedTasks)
	database.Model(&models.Task{}).Where("synced = ? AND status != ?", false, models.StatusArchived).Count(&unsyncedTasks)
	database.Model(&models.Task{}).Where("source = ? AND status != ?", models.SourceLocal, models.StatusArchived).Count(&localTasks)
	database.Model(&models.Task{}).Where("source = ? AND status != ?", models.SourceGitHub, models.StatusArchived).Count(&githubTasks)

	// Count links by direction
	var totalLinks int64
	var pushLinks int64
	var pullLinks int64

	database.Model(&models.GitHubIssueLink{}).Count(&totalLinks)
	database.Model(&models.GitHubIssueLink{}).Where("sync_direction = ?", models.SyncDirectionPush).Count(&pushLinks)
	database.Model(&models.GitHubIssueLink{}).Where("sync_direction = ?", models.SyncDirectionPull).Count(&pullLinks)

	// Get recent syncs
	var recentLinks []models.GitHubIssueLink
	database.Order("last_synced_at DESC").Limit(5).Find(&recentLinks)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"configured":     true,
			"repository":     repo,
			"total_tasks":    totalTasks,
			"synced_tasks":   syncedTasks,
			"unsynced_tasks": unsyncedTasks,
			"local_tasks":    localTasks,
			"github_tasks":   githubTasks,
			"total_links":    totalLinks,
			"push_links":     pushLinks,
			"pull_links":     pullLinks,
			"recent_syncs":   recentLinks,
		})
		return nil
	}

	fmt.Printf("GitHub Sync Status\n")
	fmt.Printf("==================\n\n")
	fmt.Printf("Repository: %s\n\n", repo)

	fmt.Printf("Tasks:\n")
	fmt.Printf("  Total:    %d\n", totalTasks)
	fmt.Printf("  Synced:   %d\n", syncedTasks)
	fmt.Printf("  Unsynced: %d\n", unsyncedTasks)
	fmt.Printf("  Local:    %d (created in gur)\n", localTasks)
	fmt.Printf("  GitHub:   %d (pulled from GitHub)\n", githubTasks)

	fmt.Printf("\nLinks:\n")
	fmt.Printf("  Total:  %d\n", totalLinks)
	fmt.Printf("  Pushed: %d (gur -> GitHub)\n", pushLinks)
	fmt.Printf("  Pulled: %d (GitHub -> gur)\n", pullLinks)

	if len(recentLinks) > 0 {
		fmt.Printf("\nRecent Syncs:\n")
		for _, link := range recentLinks {
			direction := "→"
			if link.SyncDirection == models.SyncDirectionPull {
				direction = "←"
			}
			fmt.Printf("  %s #%d %s %s (%s)\n",
				link.LastSyncedAt.Format(models.DateTimeShortFormat),
				link.IssueNumber,
				direction,
				link.TaskID,
				link.SyncDirection,
			)
		}
	}

	if unsyncedTasks > 0 {
		fmt.Printf("\nTip: Run 'gur sync push' to sync %d unsynced task(s) to GitHub.\n", unsyncedTasks)
	}

	return nil
}
