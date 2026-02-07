package cmd

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v63/github"
	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

const (
	syncMarkerPrefix = "<!-- gur-sync:"
	syncMarkerSuffix = " -->"
)

// SyncMarker represents the metadata stored in GitHub comments
type SyncMarker struct {
	TaskID   string    `json:"task_id"`
	User     string    `json:"user"`
	Machine  string    `json:"machine"`
	SyncedAt time.Time `json:"synced_at"`
}

var (
	syncPullForce  bool
	syncPullDryRun bool
	syncPullLabel  string
	syncPullAll    bool
)

var syncPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull issues from GitHub into local gur database",
	Long: `Pull GitHub issues into local gur tasks.

By default, only pulls open issues that haven't been synced yet.
Issues that were previously synced by another user will prompt for confirmation.

The command posts a sync marker comment to GitHub to coordinate with other users.`,
	RunE: runSyncPull,
}

func init() {
	syncCmd.AddCommand(syncPullCmd)

	syncPullCmd.Flags().BoolVar(&syncPullForce, "force", false, "Skip confirmation prompts for already-synced issues")
	syncPullCmd.Flags().BoolVar(&syncPullDryRun, "dry-run", false, "Show what would be pulled without actually pulling")
	syncPullCmd.Flags().StringVar(&syncPullLabel, "label", "", "Only pull issues with this label")
	syncPullCmd.Flags().BoolVar(&syncPullAll, "all", false, "Pull all issues (open and closed)")
}

func runSyncPull(cmd *cobra.Command, args []string) error {
	// Get GitHub configuration
	repo, err := db.GetConfig(models.ConfigGitHubRepo)
	if err != nil || repo == "" {
		return fmt.Errorf("GitHub not configured. Run 'gur config github' first")
	}

	token, err := GetGitHubToken()
	if err != nil {
		return err
	}

	// Parse owner/repo
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository format: %s", repo)
	}
	owner, repoName := parts[0], parts[1]

	// Create GitHub client
	httpClient := &http.Client{
		Timeout: githubAPITimeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	client := github.NewClient(httpClient).WithAuthToken(token)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Get current user info for sync marker
	currentUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	username := currentUser.GetLogin()

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	// Hash hostname for privacy - first 8 chars of SHA256
	hostnameHash := hashHostname(hostname)

	// Check if user wants to share friendly name
	machineDisplay := hostnameHash
	if name, err := db.GetConfig(models.ConfigMachineName); err == nil && name != "" {
		if share, err := db.GetConfig(models.ConfigMachineShare); err == nil && share == "true" {
			machineDisplay = fmt.Sprintf("%s (%s)", name, hostnameHash)
		}
	}

	// List issues from GitHub
	state := "open"
	if syncPullAll {
		state = "all"
	}

	opts := &github.IssueListByRepoOptions{
		State:     state,
		Sort:      "updated",
		Direction: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	if syncPullLabel != "" {
		opts.Labels = []string{syncPullLabel}
	}

	var allIssues []*github.Issue
	for {
		issues, resp, err := client.Issues.ListByRepo(ctx, owner, repoName, opts)
		if err != nil {
			return fmt.Errorf("failed to list issues: %w", err)
		}

		// Filter out pull requests (GitHub API returns PRs as issues)
		for _, issue := range issues {
			if issue.PullRequestLinks == nil {
				allIssues = append(allIssues, issue)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if len(allIssues) == 0 {
		if IsJSONOutput() {
			OutputJSON(map[string]interface{}{"success": true, "pulled": 0, "message": "No issues to pull"})
		} else {
			fmt.Println("No issues to pull")
		}
		return nil
	}

	database := db.GetDB()
	pulled := 0
	skipped := 0
	var results []map[string]interface{}

	for _, issue := range allIssues {
		issueNum := issue.GetNumber()

		// Check if already linked locally
		var existingLink models.GitHubIssueLink
		if err := database.Where("issue_number = ? AND repository = ?", issueNum, repo).First(&existingLink).Error; err == nil {
			// Already have this issue locally
			skipped++
			continue
		}

		// Check for sync marker in comments
		marker, err := findSyncMarker(ctx, client, owner, repoName, issueNum)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to check comments for issue #%d: %v\n", issueNum, err)
		}

		if marker != nil && !syncPullForce {
			// Issue was synced by someone else
			if syncPullDryRun {
				fmt.Printf("Would skip #%d \"%s\" - already synced by @%s on %s (machine: %s)\n",
					issueNum, issue.GetTitle(), marker.User, marker.SyncedAt.Format("2006-01-02"), marker.Machine)
				skipped++
				continue
			}

			// Prompt for confirmation
			fmt.Printf("\nIssue #%d \"%s\" was already synced:\n", issueNum, issue.GetTitle())
			fmt.Printf("  By: @%s\n", marker.User)
			fmt.Printf("  Date: %s\n", marker.SyncedAt.Format("2006-01-02 15:04"))
			fmt.Printf("  Machine: %s\n", marker.Machine)
			fmt.Printf("  Task ID: %s\n", marker.TaskID)
			fmt.Print("\nPull anyway? [y/N] ")

			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response != "y" && response != "yes" {
				skipped++
				continue
			}
		}

		if syncPullDryRun {
			fmt.Printf("Would pull #%d \"%s\"\n", issueNum, issue.GetTitle())
			results = append(results, map[string]interface{}{
				"issue_number": issueNum,
				"title":        issue.GetTitle(),
				"action":       "would_pull",
			})
			continue
		}

		// Create local task from GitHub issue
		task, err := createTaskFromIssue(issue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating task for issue #%d: %v\n", issueNum, err)
			continue
		}

		if err := database.Create(task).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Error saving task for issue #%d: %v\n", issueNum, err)
			continue
		}

		// Create link
		remoteUpdated := issue.GetUpdatedAt().Time
		link := models.GitHubIssueLink{
			TaskID:          task.ID,
			IssueNumber:     issueNum,
			IssueURL:        issue.GetHTMLURL(),
			Repository:      repo,
			LastSyncedAt:    time.Now(),
			RemoteUpdatedAt: &remoteUpdated,
			SyncDirection:   models.SyncDirectionPull,
			SyncedBy:        username,
			SyncedMachine:   hostnameHash,
		}
		if err := database.Create(&link).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Error saving link for issue #%d: %v\n", issueNum, err)
			continue
		}

		// Post sync marker comment to GitHub
		if err := postSyncMarker(ctx, client, owner, repoName, issueNum, task.ID, username, machineDisplay); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to post sync marker for issue #%d: %v\n", issueNum, err)
		}

		pulled++
		results = append(results, map[string]interface{}{
			"issue_number": issueNum,
			"task_id":      task.ID,
			"title":        task.Title,
			"action":       "pulled",
		})

		if !IsJSONOutput() {
			fmt.Printf("Pulled: #%d -> %s \"%s\"\n", issueNum, task.ID, task.Title)
		}
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"success": true,
			"pulled":  pulled,
			"skipped": skipped,
			"results": results,
		})
	} else if !syncPullDryRun {
		fmt.Printf("\nPulled %d issue(s), skipped %d\n", pulled, skipped)
	}

	return nil
}

func createTaskFromIssue(issue *github.Issue) (*models.Task, error) {
	task := &models.Task{
		Title:       issue.GetTitle(),
		Description: issue.GetBody(),
		Priority:    models.PriorityMedium, // Default P2
		Type:        models.TypeTask,
		Source:      models.SourceGitHub,
		Synced:      true,
	}

	// Map GitHub state to gur status
	switch issue.GetState() {
	case "closed":
		task.Status = models.StatusClosed
		task.CloseReason = "Closed on GitHub"
		now := time.Now()
		task.ClosedAt = &now
	default:
		task.Status = models.StatusOpen
	}

	// Map GitHub labels
	for _, label := range issue.Labels {
		name := strings.ToLower(label.GetName())
		task.Labels = append(task.Labels, label.GetName())

		// Infer type from labels
		if name == "bug" {
			task.Type = models.TypeBug
		} else if name == "enhancement" || name == "feature" {
			task.Type = models.TypeFeature
		}

		// Infer priority from labels
		if strings.Contains(name, "critical") || strings.Contains(name, "p0") {
			task.Priority = models.PriorityCritical
		} else if strings.Contains(name, "high") || strings.Contains(name, "p1") {
			task.Priority = models.PriorityHigh
		} else if strings.Contains(name, "low") || strings.Contains(name, "p3") {
			task.Priority = models.PriorityLow
		}
	}

	// Map assignee
	if issue.Assignee != nil {
		task.Assignee = issue.Assignee.GetLogin()
	}

	return task, nil
}

func findSyncMarker(ctx context.Context, client *github.Client, owner, repo string, issueNum int) (*SyncMarker, error) {
	opts := &github.IssueListCommentsOptions{
		Sort:      github.String("created"),
		Direction: github.String("desc"),
		ListOptions: github.ListOptions{
			PerPage: 50,
		},
	}

	comments, _, err := client.Issues.ListComments(ctx, owner, repo, issueNum, opts)
	if err != nil {
		return nil, err
	}

	// Look for sync marker in comments
	markerRegex := regexp.MustCompile(regexp.QuoteMeta(syncMarkerPrefix) + `(.+?)` + regexp.QuoteMeta(syncMarkerSuffix))

	for _, comment := range comments {
		body := comment.GetBody()
		matches := markerRegex.FindStringSubmatch(body)
		if len(matches) >= 2 {
			var marker SyncMarker
			if err := json.Unmarshal([]byte(matches[1]), &marker); err == nil {
				return &marker, nil
			}
		}
	}

	return nil, nil
}

func postSyncMarker(ctx context.Context, client *github.Client, owner, repo string, issueNum int, taskID, username, machine string) error {
	marker := SyncMarker{
		TaskID:   taskID,
		User:     username,
		Machine:  machine,
		SyncedAt: time.Now().UTC(),
	}

	markerJSON, err := json.Marshal(marker)
	if err != nil {
		return err
	}

	body := fmt.Sprintf(`🤖 **Synced to local gur database**
- User: @%s
- Date: %s
- Machine: %s
- Task ID: %s

%s%s%s`,
		username,
		marker.SyncedAt.Format("2006-01-02 15:04 UTC"),
		machine,
		taskID,
		syncMarkerPrefix,
		string(markerJSON),
		syncMarkerSuffix,
	)

	comment := &github.IssueComment{Body: &body}
	_, _, err = client.Issues.CreateComment(ctx, owner, repo, issueNum, comment)
	return err
}

// hashHostname creates a short hash of the hostname for privacy
func hashHostname(hostname string) string {
	h := sha256.Sum256([]byte(hostname))
	return hex.EncodeToString(h[:])[:8] // First 8 hex chars
}
