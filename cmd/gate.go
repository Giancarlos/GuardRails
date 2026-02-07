package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	gateCategory    string
	gateType        string
	gatePriority    int
	gateLabels      []string
	gatePrecond     string
	gateSteps       string
	gateExpected    string
	gateCommand     string
	gateDescription string
)

var gateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Quality gate management",
	Long: `Manage quality gates for your project.

Gates are requirements that must pass before a task can be closed.
They can be tests, reviews, approvals, or any custom verification.

COMMON TYPES: test, review, approval, manual, deploy, qa, doc (or any custom type)
RESULTS: pending, passed, failed, skipped`,
}

var gateCreateCmd = &cobra.Command{
	Use:   "create \"title\"",
	Short: "Create a new gate",
	Long: `Create a new quality gate.

SUGGESTED TYPES (or use any custom type):
  test       - Automated or manual test
  review     - Code review
  approval   - Sign-off from someone
  manual     - Manual verification
  deploy     - Deployment check
  qa         - QA verification
  security   - Security scan/review
  doc        - Documentation check

Examples:
  gur gate create "Unit tests pass" -t test -c backend
  gur gate create "Code review approved" -t review
  gur gate create "PM sign-off" -t approval
  gur gate create "Security scan" -t security --cmd "npm audit"`,
	Args: cobra.ExactArgs(1),
	RunE: runGateCreate,
}

var gateListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List gates",
	Aliases: []string{"ls"},
	RunE:    runGateList,
}

var gateShowCmd = &cobra.Command{
	Use:   "show <gate-id>",
	Short: "Show gate details",
	Args:  cobra.ExactArgs(1),
	RunE:  runGateShow,
}

var gatePassCmd = &cobra.Command{
	Use:   "pass <gate-id> <task-id>",
	Short: "Mark a gate as passed for a specific task",
	Long: `Mark a gate as passed for a specific task.

Each task requires its own gate verification - you cannot reuse a previous pass.

Examples:
  gur gate pass gate-abc123 gur-def456
  gur gate pass gate-abc123 gur-def456 --notes "All tests green"
  gur gate pass gate-abc123 gur-def456 --by agent`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGateResult(args[0], args[1], models.GateLinkPassed)
	},
}

var gateFailCmd = &cobra.Command{
	Use:   "fail <gate-id> <task-id>",
	Short: "Mark a gate as failed for a specific task",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGateResult(args[0], args[1], models.GateLinkFailed)
	},
}

var gateSkipCmd = &cobra.Command{
	Use:   "skip <gate-id> <task-id>",
	Short: "Mark a gate as skipped for a specific task (still blocks close)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runGateResult(args[0], args[1], models.GateSkipped)
	},
}

var gateLinkCmd = &cobra.Command{
	Use:   "link <gate-id> <task-id>",
	Short: "Link a gate to a task",
	Long: `Link a gate to a task as a requirement.

The task cannot be closed until this gate passes.

Example:
  gur gate link gate-abc123 gur-def456`,
	Args: cobra.ExactArgs(2),
	RunE: runGateLink,
}

var gateUnlinkCmd = &cobra.Command{
	Use:   "unlink <gate-id> <task-id>",
	Short: "Unlink a gate from a task",
	Args:  cobra.ExactArgs(2),
	RunE:  runGateUnlink,
}

var (
	gateNotes string
	gateRunBy string
)

func init() {
	rootCmd.AddCommand(gateCmd)
	gateCmd.AddCommand(gateCreateCmd)
	gateCmd.AddCommand(gateListCmd)
	gateCmd.AddCommand(gateShowCmd)
	gateCmd.AddCommand(gatePassCmd)
	gateCmd.AddCommand(gateFailCmd)
	gateCmd.AddCommand(gateSkipCmd)
	gateCmd.AddCommand(gateLinkCmd)
	gateCmd.AddCommand(gateUnlinkCmd)

	// Create flags
	gateCreateCmd.Flags().StringVarP(&gateCategory, "category", "c", "", "Category (e.g., auth, api, ui)")
	gateCreateCmd.Flags().StringVarP(&gateType, "type", "t", "manual", "Type (e.g., test, review, approval, manual)")
	gateCreateCmd.Flags().IntVarP(&gatePriority, "priority", "p", 2, "Priority (0-4)")
	gateCreateCmd.Flags().StringArrayVarP(&gateLabels, "label", "l", nil, "Labels")
	gateCreateCmd.Flags().StringVar(&gatePrecond, "pre", "", "Preconditions")
	gateCreateCmd.Flags().StringVar(&gateSteps, "steps", "", "Steps to verify")
	gateCreateCmd.Flags().StringVar(&gateExpected, "expected", "", "Expected result")
	gateCreateCmd.Flags().StringVar(&gateCommand, "cmd", "", "Command to run (for automated gates)")
	gateCreateCmd.Flags().StringVarP(&gateDescription, "description", "d", "", "Description")

	// List flags
	gateListCmd.Flags().StringVarP(&gateCategory, "category", "c", "", "Filter by category")
	gateListCmd.Flags().StringVarP(&gateType, "type", "t", "", "Filter by type")
	gateListCmd.Flags().StringVar(&listStatus, "result", "", "Filter by last result")

	// Pass/fail/skip flags
	gatePassCmd.Flags().StringVar(&gateNotes, "notes", "", "Notes about the result")
	gatePassCmd.Flags().StringVar(&gateRunBy, "by", "human", "Who verified (human/agent/name)")
	gateFailCmd.Flags().StringVar(&gateNotes, "notes", "", "Notes about the result")
	gateFailCmd.Flags().StringVar(&gateRunBy, "by", "human", "Who verified (human/agent/name)")
	gateSkipCmd.Flags().StringVar(&gateNotes, "notes", "", "Notes about the result")
	gateSkipCmd.Flags().StringVar(&gateRunBy, "by", "human", "Who verified (human/agent/name)")
}

func runGateCreate(cmd *cobra.Command, args []string) error {
	gate := &models.Gate{
		Title:          args[0],
		Description:    gateDescription,
		Category:       gateCategory,
		Type:           gateType,
		Priority:       gatePriority,
		Preconditions:  gatePrecond,
		Steps:          gateSteps,
		ExpectedResult: gateExpected,
		Command:        gateCommand,
		Labels:         gateLabels,
		LastResult:     models.GatePending,
	}

	if err := db.GetDB().Create(gate).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "gate": gate})
	} else {
		fmt.Printf("Created: %s - %s\n", gate.ID, gate.Title)
		if gate.Category != "" {
			fmt.Printf("  Category: %s\n", gate.Category)
		}
		fmt.Printf("  Type: %s\n", gate.TypeString())
	}
	return nil
}

func runGateList(cmd *cobra.Command, args []string) error {
	var gates []models.Gate
	query := db.GetDB().Order("priority ASC, category ASC, created_at DESC")

	if gateCategory != "" {
		query = query.Where("category = ?", gateCategory)
	}
	if gateType != "" {
		query = query.Where("type = ?", gateType)
	}
	if listStatus != "" {
		query = query.Where("last_result = ?", listStatus)
	}

	if err := query.Find(&gates).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"count": len(gates), "gates": gates})
		return nil
	}

	if len(gates) == 0 {
		fmt.Println("No gates found")
		return nil
	}

	for _, g := range gates {
		cat := ""
		if g.Category != "" {
			cat = "[" + g.Category + "] "
		}
		fmt.Printf("[%s] %s%s - %s (%s)\n", g.ID, cat, g.ResultString(), g.Title, g.TypeString())
	}
	return nil
}

func runGateShow(cmd *cobra.Command, args []string) error {
	gate, err := db.GetGateByID(args[0])
	if err != nil {
		return fmt.Errorf("gate '%s' not found (use 'gur gate list' to see available gates)", args[0])
	}

	// Get linked tasks
	var links []models.GateTaskLink
	db.GetDB().Where("gate_id = ?", gate.ID).Find(&links)

	// Get recent runs
	var runs []models.GateRun
	db.GetDB().Where("gate_id = ?", gate.ID).Order("created_at DESC").Limit(5).Find(&runs)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"gate":         gate,
			"linked_tasks": links,
			"recent_runs":  runs,
		})
		return nil
	}

	fmt.Printf("ID:       %s\n", gate.ID)
	fmt.Printf("Title:    %s\n", gate.Title)
	fmt.Printf("Type:     %s\n", gate.TypeString())
	fmt.Printf("Priority: P%d\n", gate.Priority)
	fmt.Printf("Result:   %s\n", gate.ResultString())
	if gate.Category != "" {
		fmt.Printf("Category: %s\n", gate.Category)
	}
	if gate.Description != "" {
		fmt.Printf("Desc:     %s\n", gate.Description)
	}
	if gate.Preconditions != "" {
		fmt.Printf("\nPreconditions:\n%s\n", gate.Preconditions)
	}
	if gate.Steps != "" {
		fmt.Printf("\nSteps:\n%s\n", gate.Steps)
	}
	if gate.ExpectedResult != "" {
		fmt.Printf("\nExpected:\n%s\n", gate.ExpectedResult)
	}
	if gate.Command != "" {
		fmt.Printf("\nCommand: %s\n", gate.Command)
	}
	if len(gate.Labels) > 0 {
		fmt.Printf("Labels:   %v\n", gate.Labels)
	}

	fmt.Printf("\nStats: %d runs, %d passed, %d failed (%.0f%% pass rate)\n",
		gate.RunCount, gate.PassCount, gate.FailCount, gate.PassRate())

	if len(links) > 0 {
		fmt.Println("\nLinked tasks:")
		for _, l := range links {
			status := l.Status
			if status == "" {
				status = "pending"
			}
			fmt.Printf("  %s (%s)\n", l.TaskID, status)
		}
	}

	if len(runs) > 0 {
		fmt.Println("\nRecent runs:")
		for _, r := range runs {
			fmt.Printf("  %s - %s by %s\n", r.CreatedAt.Format(models.DateTimeShortFormat), r.Result, r.RunBy)
			if r.Notes != "" {
				fmt.Printf("    Notes: %s\n", r.Notes)
			}
		}
	}

	return nil
}

func runGateResult(gateID string, taskID string, result string) error {
	database := db.GetDB()

	// Validate gate exists
	gate, err := db.GetGateByID(gateID)
	if err != nil {
		return fmt.Errorf("cannot update gate: gate '%s' not found (use 'gur gate list' to see available gates)", gateID)
	}

	// Validate task exists
	task, err := db.GetTaskByID(taskID)
	if err != nil {
		return fmt.Errorf("cannot update gate: task '%s' not found (use 'gur list' to see available tasks)", taskID)
	}

	// Find the link between gate and task
	var link models.GateTaskLink
	err = database.Where("gate_id = ? AND task_id = ?", gateID, taskID).First(&link).Error
	if err != nil {
		return fmt.Errorf("cannot update gate: gate '%s' is not linked to task '%s'\nLink it first: gur gate link %s %s", gateID, taskID, gateID, taskID)
	}

	// Update the per-task link status
	now := time.Now()
	link.Status = result
	link.VerifiedAt = &now
	link.VerifiedBy = gateRunBy
	link.Notes = gateNotes
	if err := database.Save(&link).Error; err != nil {
		return fmt.Errorf("failed to update gate link: %w", err)
	}

	// Also update global gate stats and save to GateRun history for audit
	gate.RecordRun(result, gateRunBy, gateNotes)
	if err := database.Save(&gate).Error; err != nil {
		return fmt.Errorf("failed to update gate stats: %w", err)
	}

	run := &models.GateRun{
		GateID: gateID,
		Result: result,
		RunBy:  gateRunBy,
		Notes:  gateNotes,
	}
	if err := database.Create(run).Error; err != nil {
		return fmt.Errorf("failed to save gate run history: %w", err)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "gate": gate, "task": task, "link": link})
	} else {
		fmt.Printf("Verified: %s for task %s (%s by %s)\n", gate.Title, taskID, result, gateRunBy)
	}
	return nil
}

func runGateLink(cmd *cobra.Command, args []string) error {
	gateID, taskID := args[0], args[1]
	database := db.GetDB()

	// Validate gate exists
	if _, err := db.GetGateByID(gateID); err != nil {
		return fmt.Errorf("cannot link gate: gate '%s' not found (use 'gur gate list' to see available gates)", gateID)
	}

	// Validate task exists
	if _, err := db.GetTaskByID(taskID); err != nil {
		return fmt.Errorf("cannot link gate: task '%s' not found (use 'gur list' to see available tasks)", taskID)
	}

	// Check if already linked
	var existing models.GateTaskLink
	err := database.Where("gate_id = ? AND task_id = ?", gateID, taskID).First(&existing).Error
	if err == nil {
		return fmt.Errorf("cannot link gate: gate '%s' is already linked to task '%s'", gateID, taskID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("cannot link gate: failed to check existing link: %w", err)
	}

	link := &models.GateTaskLink{
		GateID: gateID,
		TaskID: taskID,
		Status: models.GateLinkPending,
	}
	if err := database.Create(link).Error; err != nil {
		return fmt.Errorf("failed to link gate '%s' to task '%s': database error: %w", gateID, taskID, err)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "link": link})
	} else {
		fmt.Printf("Linked: %s -> %s (status: pending)\n", gateID, taskID)
		fmt.Println("Task cannot be closed until this gate is verified for this task.")
		fmt.Printf("Verify with: gur gate pass %s %s\n", gateID, taskID)
	}
	return nil
}

func runGateUnlink(cmd *cobra.Command, args []string) error {
	gateID, taskID := args[0], args[1]

	result := db.GetDB().Where("gate_id = ? AND task_id = ?", gateID, taskID).Delete(&models.GateTaskLink{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("cannot unlink gate: no link exists between gate '%s' and task '%s' (use 'gur gate show %s' to see linked tasks)",
			gateID, taskID, gateID)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true})
	} else {
		fmt.Println("Unlinked gate from task")
	}
	return nil
}

// GateLinkInfo contains gate info with its per-task link status
type GateLinkInfo struct {
	Gate   models.Gate
	Link   models.GateTaskLink
	Status string
}

// GetGateLinksForTask returns all gate links for a task with their per-task status
func GetGateLinksForTask(taskID string) ([]GateLinkInfo, error) {
	database := db.GetDB()

	var links []models.GateTaskLink
	if err := database.Where("task_id = ? AND deleted_at IS NULL", taskID).Find(&links).Error; err != nil {
		return nil, err
	}

	var result []GateLinkInfo
	for _, link := range links {
		gate, err := db.GetGateByID(link.GateID)
		if err != nil {
			continue
		}
		result = append(result, GateLinkInfo{
			Gate:   *gate,
			Link:   link,
			Status: link.Status,
		})
	}

	return result, nil
}

// GetFailingGateLinksForTask returns gates linked to a task where the per-task status is not "passed"
func GetFailingGateLinksForTask(taskID string) ([]GateLinkInfo, error) {
	links, err := GetGateLinksForTask(taskID)
	if err != nil {
		return nil, err
	}

	var failing []GateLinkInfo
	for _, info := range links {
		if info.Status != models.GateLinkPassed {
			failing = append(failing, info)
		}
	}

	return failing, nil
}

// GetLinkedGatesForTask returns all gates linked to a task
func GetLinkedGatesForTask(taskID string) ([]models.Gate, error) {
	database := db.GetDB()

	var gates []models.Gate
	err := database.
		Joins("JOIN gate_task_links ON gate_task_links.gate_id = gates.id").
		Where("gate_task_links.task_id = ? AND gate_task_links.deleted_at IS NULL", taskID).
		Find(&gates).Error

	if err != nil {
		return nil, err
	}

	return gates, nil
}

// CheckGatesBeforeClose checks if all linked gates have been verified as passed for this specific task.
// Tasks MUST have at least one gate linked to be closed.
// Each gate must be verified per-task - global gate status is not sufficient.
func CheckGatesBeforeClose(taskID string) error {
	gateLinks, err := GetGateLinksForTask(taskID)
	if err != nil {
		return err
	}

	// Require at least one gate to be linked
	if len(gateLinks) == 0 {
		return fmt.Errorf("Cannot close task: no gates linked.\n\nEvery task must have at least one gate before closing.\nLink a gate: gur gate link <gate-id> %s\nOr use --force to close anyway (requires interactive confirmation).", taskID)
	}

	failingLinks, err := GetFailingGateLinksForTask(taskID)
	if err != nil {
		return err
	}

	if len(failingLinks) > 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Cannot close task: %d gate(s) not verified for this task:\n", len(failingLinks)))
		for _, info := range failingLinks {
			status := info.Status
			if status == "" {
				status = "pending"
			}
			sb.WriteString(fmt.Sprintf("  - %s: %s (status: %s)\n", info.Gate.ID, info.Gate.Title, status))
		}
		sb.WriteString(fmt.Sprintf("\nVerify gates for this task:\n"))
		for _, info := range failingLinks {
			sb.WriteString(fmt.Sprintf("  gur gate pass %s %s\n", info.Gate.ID, taskID))
		}
		sb.WriteString("\nOr use --force to close anyway (requires interactive confirmation).")
		return fmt.Errorf("%s", sb.String())
	}

	return nil
}
