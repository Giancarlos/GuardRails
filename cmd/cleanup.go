package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	cleanupDryRun bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up orphaned records from deleted tasks",
	Long: `Remove orphaned dependencies and link records that reference deleted tasks.

This is useful for database maintenance after tasks have been deleted.
The cleanup is performed in a transaction to ensure data consistency.

Examples:
  gur cleanup            # Clean up all orphaned records
  gur cleanup --dry-run  # Show what would be cleaned without making changes`,
	RunE: runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Show what would be cleaned without making changes")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	database := db.GetDB()

	// Count orphaned records before cleanup
	var orphanedDeps int64
	var orphanedGateLinks int64
	var orphanedSkillLinks int64
	var orphanedAgentLinks int64
	var orphanedGitHubLinks int64

	// Orphaned dependencies: where parent_id or child_id references a deleted task
	database.Model(&models.Dependency{}).
		Where("parent_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
		Or("child_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
		Count(&orphanedDeps)

	// Orphaned gate links: where task_id references a deleted task
	database.Model(&models.GateTaskLink{}).
		Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
		Count(&orphanedGateLinks)

	// Orphaned skill links: where task_id references a deleted task
	database.Model(&models.TaskSkillLink{}).
		Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
		Count(&orphanedSkillLinks)

	// Orphaned agent links: where task_id references a deleted task
	database.Model(&models.TaskAgentLink{}).
		Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
		Count(&orphanedAgentLinks)

	// Orphaned GitHub issue links: where task_id references a deleted task
	database.Model(&models.GitHubIssueLink{}).
		Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
		Count(&orphanedGitHubLinks)

	totalOrphaned := orphanedDeps + orphanedGateLinks + orphanedSkillLinks + orphanedAgentLinks + orphanedGitHubLinks

	if totalOrphaned == 0 {
		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{
				"message":        "No orphaned records found",
				"cleaned_counts": map[string]int64{},
			})
			return nil
		}
		fmt.Println("No orphaned records found")
		return nil
	}

	if cleanupDryRun {
		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{
				"dry_run": true,
				"orphaned_counts": map[string]int64{
					"dependencies":     orphanedDeps,
					"gate_links":       orphanedGateLinks,
					"skill_links":      orphanedSkillLinks,
					"agent_links":      orphanedAgentLinks,
					"github_links":     orphanedGitHubLinks,
					"total":            totalOrphaned,
				},
			})
			return nil
		}
		fmt.Println("=== Dry Run: Orphaned Records Found ===")
		fmt.Printf("  Dependencies:       %d\n", orphanedDeps)
		fmt.Printf("  Gate Links:         %d\n", orphanedGateLinks)
		fmt.Printf("  Skill Links:        %d\n", orphanedSkillLinks)
		fmt.Printf("  Agent Links:        %d\n", orphanedAgentLinks)
		fmt.Printf("  GitHub Issue Links: %d\n", orphanedGitHubLinks)
		fmt.Printf("  ---\n")
		fmt.Printf("  Total:              %d\n", totalOrphaned)
		fmt.Println("\nRun without --dry-run to remove these records")
		return nil
	}

	// Perform cleanup in a transaction
	var cleanedDeps, cleanedGateLinks, cleanedSkillLinks, cleanedAgentLinks, cleanedGitHubLinks int64

	err := database.Transaction(func(tx *gorm.DB) error {
		// Clean orphaned dependencies
		result := tx.Where("parent_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
			Or("child_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
			Delete(&models.Dependency{})
		if result.Error != nil {
			return result.Error
		}
		cleanedDeps = result.RowsAffected

		// Clean orphaned gate links
		result = tx.Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
			Delete(&models.GateTaskLink{})
		if result.Error != nil {
			return result.Error
		}
		cleanedGateLinks = result.RowsAffected

		// Clean orphaned skill links
		result = tx.Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
			Delete(&models.TaskSkillLink{})
		if result.Error != nil {
			return result.Error
		}
		cleanedSkillLinks = result.RowsAffected

		// Clean orphaned agent links
		result = tx.Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
			Delete(&models.TaskAgentLink{})
		if result.Error != nil {
			return result.Error
		}
		cleanedAgentLinks = result.RowsAffected

		// Clean orphaned GitHub issue links
		result = tx.Where("task_id NOT IN (SELECT id FROM tasks WHERE deleted_at IS NULL)").
			Delete(&models.GitHubIssueLink{})
		if result.Error != nil {
			return result.Error
		}
		cleanedGitHubLinks = result.RowsAffected

		return nil
	})

	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	totalCleaned := cleanedDeps + cleanedGateLinks + cleanedSkillLinks + cleanedAgentLinks + cleanedGitHubLinks

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"success": true,
			"cleaned_counts": map[string]int64{
				"dependencies":     cleanedDeps,
				"gate_links":       cleanedGateLinks,
				"skill_links":      cleanedSkillLinks,
				"agent_links":      cleanedAgentLinks,
				"github_links":     cleanedGitHubLinks,
				"total":            totalCleaned,
			},
		})
		return nil
	}

	fmt.Println("=== Cleanup Complete ===")
	fmt.Printf("  Dependencies:       %d removed\n", cleanedDeps)
	fmt.Printf("  Gate Links:         %d removed\n", cleanedGateLinks)
	fmt.Printf("  Skill Links:        %d removed\n", cleanedSkillLinks)
	fmt.Printf("  Agent Links:        %d removed\n", cleanedAgentLinks)
	fmt.Printf("  GitHub Issue Links: %d removed\n", cleanedGitHubLinks)
	fmt.Printf("  ---\n")
	fmt.Printf("  Total:              %d removed\n", totalCleaned)

	return nil
}
