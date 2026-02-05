package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v63/github"
	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

const (
	// GitHub API timeout for individual requests
	githubAPITimeout = 30 * time.Second
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync tasks with external systems",
}

var syncPushCmd = &cobra.Command{
	Use:   "push [task-id]",
	Short: "Push tasks to GitHub Issues",
	Long: `Push gur tasks to GitHub Issues.

If no task ID is provided, pushes all open tasks that haven't been synced yet.
If a task ID is provided, pushes or updates that specific task.

Tasks that have already been synced will be updated on GitHub.
New tasks will create new GitHub issues.

The issue title will be prefixed with the configured prefix (default: "[Coding Agent]").`,
	RunE: runSyncPush,
}

var (
	syncPushAll    bool
	syncPushOpen   bool
	syncPushClosed bool
	syncPushDryRun bool
)

func init() {
	rootCmd.AddCommand(syncCmd)
	syncCmd.AddCommand(syncPushCmd)

	syncPushCmd.Flags().BoolVar(&syncPushAll, "all", false, "Push all tasks (open and closed)")
	syncPushCmd.Flags().BoolVar(&syncPushOpen, "open", false, "Push only open tasks")
	syncPushCmd.Flags().BoolVar(&syncPushClosed, "closed", false, "Push only closed tasks")
	syncPushCmd.Flags().BoolVar(&syncPushDryRun, "dry-run", false, "Show what would be pushed without actually pushing")
}

func runSyncPush(cmd *cobra.Command, args []string) error {
	// Get GitHub configuration
	repo, err := db.GetConfig(models.ConfigGitHubRepo)
	if err != nil || repo == "" {
		return fmt.Errorf("GitHub sync not configured: repository not set (run 'gur config github' to configure)")
	}

	prefix, err := db.GetConfig(models.ConfigGitHubIssuePrefix)
	if err != nil || prefix == "" {
		prefix = models.DefaultGitHubIssuePrefix
	}

	token, err := GetGitHubToken()
	if err != nil {
		return err
	}

	// Parse owner/repo
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format '%s': expected 'owner/repo' (run 'gur config github' to reconfigure)", repo)
	}
	owner, repoName := parts[0], parts[1]

	// Create GitHub client with connection pooling
	httpClient := &http.Client{
		Timeout: githubAPITimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	client := github.NewClient(httpClient).WithAuthToken(token)

	// Create context with timeout for the entire sync operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	database := db.GetDB()

	// Determine which tasks to push
	var tasks []models.Task
	if len(args) > 0 {
		// Push specific task
		task, err := db.GetTaskByID(args[0])
		if err != nil {
			return fmt.Errorf("cannot sync task: task '%s' not found (use 'gur list' to see available tasks)", args[0])
		}
		tasks = append(tasks, *task)
	} else if syncPushAll {
		// Push all tasks (open and closed, excluding archived)
		if err := database.Where("status != ?", models.StatusArchived).
			Where("synced = ?", false).
			Find(&tasks).Error; err != nil {
			return err
		}
	} else if syncPushClosed {
		// Push only closed tasks
		if err := database.Where("status = ?", models.StatusClosed).
			Where("synced = ?", false).
			Find(&tasks).Error; err != nil {
			return err
		}
	} else if syncPushOpen {
		// Push only open tasks
		if err := database.Where("status NOT IN ?", []string{models.StatusArchived, models.StatusClosed}).
			Where("synced = ?", false).
			Find(&tasks).Error; err != nil {
			return err
		}
	} else {
		// Default: push unsynced open tasks (same as --open)
		if err := database.Where("status NOT IN ?", []string{models.StatusArchived, models.StatusClosed}).
			Where("synced = ?", false).
			Find(&tasks).Error; err != nil {
			return err
		}
	}

	if len(tasks) == 0 {
		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{"success": true, "synced": 0, "message": "No tasks to sync"})
		} else {
			fmt.Println("No tasks to sync")
		}
		return nil
	}

	if syncPushDryRun {
		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{"dry_run": true, "tasks": tasks})
		} else {
			fmt.Printf("Would push %d task(s):\n", len(tasks))
			for _, t := range tasks {
				fmt.Printf("  [%s] %s\n", t.ID, t.Title)
			}
		}
		return nil
	}

	var results []map[string]interface{}
	synced := 0
	errors := 0

	for _, task := range tasks {
		result, err := syncTaskToGitHub(ctx, client, owner, repoName, prefix, task)
		if err != nil {
			errors++
			result = map[string]interface{}{
				"task_id": task.ID,
				"error":   err.Error(),
			}
			if !IsJSONOutput() {
				fmt.Printf("Error syncing %s: %v\n", task.ID, err)
			}
		} else {
			synced++
			if !IsJSONOutput() {
				fmt.Printf("Synced: %s -> %s\n", task.ID, result["issue_url"])
			}
		}
		results = append(results, result)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"success": errors == 0,
			"synced":  synced,
			"errors":  errors,
			"results": results,
		})
	} else if synced > 0 {
		fmt.Printf("\nSynced %d task(s) to GitHub\n", synced)
		if errors > 0 {
			fmt.Printf("%d task(s) failed to sync\n", errors)
		}
	}

	return nil
}

func syncTaskToGitHub(ctx context.Context, client *github.Client, owner, repo, prefix string, task models.Task) (map[string]interface{}, error) {
	database := db.GetDB()

	// Check if task already has a GitHub issue
	var link models.GitHubIssueLink
	existingLink := database.Where("task_id = ?", task.ID).First(&link).Error == nil

	// Build issue title and body
	title := fmt.Sprintf("%s - %s", prefix, task.Title)
	body := buildIssueBody(task)

	if existingLink {
		// Update existing issue
		state := mapStatusToGitHub(task.Status)
		issueRequest := &github.IssueRequest{
			Title: &title,
			Body:  &body,
			State: &state,
		}

		issue, _, err := client.Issues.Edit(ctx, owner, repo, link.IssueNumber, issueRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to update issue: %w", err)
		}

		// Update link
		link.LastSyncedAt = time.Now()
		if err := database.Save(&link).Error; err != nil {
			return nil, fmt.Errorf("failed to update link: %w", err)
		}

		return map[string]interface{}{
			"task_id":      task.ID,
			"issue_number": issue.GetNumber(),
			"issue_url":    issue.GetHTMLURL(),
			"action":       "updated",
		}, nil
	}

	// Create new issue
	issueRequest := &github.IssueRequest{
		Title: &title,
		Body:  &body,
	}

	// Add labels based on task type and priority
	labels := buildLabels(task)
	if len(labels) > 0 {
		issueRequest.Labels = &labels
	}

	issue, _, err := client.Issues.Create(ctx, owner, repo, issueRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	// If task is closed, close the issue immediately
	if task.IsClosed() {
		state := "closed"
		closeRequest := &github.IssueRequest{State: &state}
		issue, _, err = client.Issues.Edit(ctx, owner, repo, issue.GetNumber(), closeRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to close issue: %w", err)
		}
	}

	// Create link
	newLink := models.GitHubIssueLink{
		TaskID:       task.ID,
		IssueNumber:  issue.GetNumber(),
		IssueURL:     issue.GetHTMLURL(),
		Repository:   fmt.Sprintf("%s/%s", owner, repo),
		LastSyncedAt: time.Now(),
	}
	if err := database.Create(&newLink).Error; err != nil {
		return nil, fmt.Errorf("failed to save link: %w", err)
	}

	// Mark task as synced
	if err := database.Model(&models.Task{}).Where("id = ?", task.ID).Update("synced", true).Error; err != nil {
		return nil, fmt.Errorf("failed to mark task as synced: %w", err)
	}

	return map[string]interface{}{
		"task_id":      task.ID,
		"issue_number": issue.GetNumber(),
		"issue_url":    issue.GetHTMLURL(),
		"action":       "created",
	}, nil
}

func buildIssueBody(task models.Task) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("**Task ID:** `%s`\n\n", task.ID))

	if task.Description != "" {
		sb.WriteString("## Description\n\n")
		sb.WriteString(task.Description)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Details\n\n")
	sb.WriteString("| Field | Value |\n")
	sb.WriteString("| ----- | ----- |\n")
	sb.WriteString(fmt.Sprintf("| Priority | %s |\n", task.PriorityString()))
	sb.WriteString(fmt.Sprintf("| Type | %s |\n", task.Type))
	sb.WriteString(fmt.Sprintf("| Status | %s |\n", task.Status))

	if task.Assignee != "" {
		sb.WriteString(fmt.Sprintf("| Assignee | %s |\n", task.Assignee))
	}

	if len(task.Labels) > 0 {
		sb.WriteString(fmt.Sprintf("| Labels | %s |\n", strings.Join(task.Labels, ", ")))
	}

	sb.WriteString(fmt.Sprintf("| Created | %s |\n", task.CreatedAt.Format(models.DateTimeShortFormat)))

	if task.Notes != "" {
		sb.WriteString("\n## Notes\n\n")
		sb.WriteString("```\n")
		sb.WriteString(task.Notes)
		sb.WriteString("```\n")
	}

	sb.WriteString("\n---\n")
	sb.WriteString("*Synced from [GuardRails](https://github.com/Giancarlos/GuardRails) task management*")

	return sb.String()
}

func buildLabels(task models.Task) []string {
	var labels []string

	// Add type as label
	switch task.Type {
	case models.TypeBug:
		labels = append(labels, "bug")
	case models.TypeFeature:
		labels = append(labels, "enhancement")
	case models.TypeEpic:
		labels = append(labels, "epic")
	}

	// Add priority as label
	switch task.Priority {
	case models.PriorityCritical:
		labels = append(labels, "priority: critical")
	case models.PriorityHigh:
		labels = append(labels, "priority: high")
	}

	// Add agent label
	labels = append(labels, "agent-created")

	return labels
}

func mapStatusToGitHub(status string) string {
	switch status {
	case models.StatusClosed, models.StatusArchived:
		return "closed"
	default:
		return "open"
	}
}
