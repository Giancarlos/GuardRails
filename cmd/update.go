package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	updateTitle       string
	updateDescription string
	updatePriority    int
	updateType        string
	updateStatus      string
	updateAssignee    string
	updateNotes       string
	updateAddLabel    []string
	updateRemoveLabel []string
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVar(&updateTitle, "title", "", "New title")
	updateCmd.Flags().StringVar(&updateDescription, "description", "", "New description")
	updateCmd.Flags().IntVarP(&updatePriority, "priority", "p", -1, "New priority")
	updateCmd.Flags().StringVarP(&updateType, "type", "t", "", "New type")
	updateCmd.Flags().StringVarP(&updateStatus, "status", "s", "", "New status")
	updateCmd.Flags().StringVarP(&updateAssignee, "assignee", "a", "", "New assignee")
	updateCmd.Flags().StringVar(&updateNotes, "notes", "", "Append notes")
	updateCmd.Flags().StringArrayVar(&updateAddLabel, "label", nil, "Add label")
	updateCmd.Flags().StringArrayVar(&updateRemoveLabel, "remove-label", nil, "Remove label")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	var task models.Task
	if err := db.GetDB().Where("id = ?", args[0]).First(&task).Error; err != nil {
		return fmt.Errorf("task not found: %s", args[0])
	}

	// Prevent modifying closed tasks (except reopening via 'reopen' command)
	if task.IsClosed() && cmd.Flags().Changed("status") && updateStatus != models.StatusClosed {
		return fmt.Errorf("cannot change status of closed task. Use 'gur reopen' to reopen it first")
	}

	// Track changes for audit trail
	database := db.GetDB()
	changedBy := "user" // Could be enhanced to track actual user

	if cmd.Flags().Changed("title") {
		models.RecordChange(database, task.ID, "title", task.Title, updateTitle, changedBy)
		task.Title = updateTitle
	}
	if cmd.Flags().Changed("description") {
		models.RecordChange(database, task.ID, "description", task.Description, updateDescription, changedBy)
		task.Description = updateDescription
	}
	if cmd.Flags().Changed("priority") {
		// Validate priority range
		if updatePriority < 0 || updatePriority > 4 {
			return fmt.Errorf("priority must be between 0 and 4")
		}
		models.RecordChange(database, task.ID, "priority", fmt.Sprintf("%d", task.Priority), fmt.Sprintf("%d", updatePriority), changedBy)
		task.Priority = updatePriority
	}
	if cmd.Flags().Changed("type") {
		models.RecordChange(database, task.ID, "type", task.Type, updateType, changedBy)
		task.Type = updateType
	}
	if cmd.Flags().Changed("status") {
		// Validate status values
		validStatuses := map[string]bool{
			models.StatusOpen:       true,
			models.StatusInProgress: true,
			models.StatusClosed:     true,
		}
		if !validStatuses[updateStatus] {
			return fmt.Errorf("invalid status: %s (must be open/in_progress/closed)", updateStatus)
		}
		models.RecordChange(database, task.ID, "status", task.Status, updateStatus, changedBy)
		task.Status = updateStatus
	}
	if cmd.Flags().Changed("assignee") {
		models.RecordChange(database, task.ID, "assignee", task.Assignee, updateAssignee, changedBy)
		task.Assignee = updateAssignee
	}
	if cmd.Flags().Changed("notes") {
		models.RecordChange(database, task.ID, "notes", "", updateNotes, changedBy)
		task.AppendNotes(updateNotes)
	}
	for _, l := range updateAddLabel {
		models.RecordChange(database, task.ID, "label_added", "", l, changedBy)
		task.AddLabel(l)
	}
	for _, l := range updateRemoveLabel {
		models.RecordChange(database, task.ID, "label_removed", l, "", changedBy)
		task.RemoveLabel(l)
	}

	if err := database.Save(&task).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "task": task})
	} else {
		fmt.Printf("Updated: %s\n", task.ID)
	}
	return nil
}
