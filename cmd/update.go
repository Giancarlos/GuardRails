package cmd

import (
	"fmt"
	"os"

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
	updateAddSkill    []string
	updateRemoveSkill []string
	updateAddAgent    []string
	updateRemoveAgent []string
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
	updateCmd.Flags().StringArrayVar(&updateAddSkill, "skill", nil, "Link skill to task")
	updateCmd.Flags().StringArrayVar(&updateRemoveSkill, "remove-skill", nil, "Unlink skill from task")
	updateCmd.Flags().StringArrayVar(&updateAddAgent, "agent", nil, "Link agent to task")
	updateCmd.Flags().StringArrayVar(&updateRemoveAgent, "remove-agent", nil, "Unlink agent from task")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	var task models.Task
	if err := db.GetDB().Where("id = ?", args[0]).First(&task).Error; err != nil {
		return fmt.Errorf("cannot update task: task '%s' not found (use 'gur list' to see available tasks)", args[0])
	}

	// Prevent modifying closed tasks (except reopening via 'reopen' command)
	if task.IsClosed() && cmd.Flags().Changed("status") && updateStatus != models.StatusClosed {
		return fmt.Errorf("cannot change status of closed task '%s': use 'gur reopen %s' first", task.ID, task.ID)
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
			return fmt.Errorf("invalid priority %d for task '%s': must be 0 (critical) to 4 (lowest)", updatePriority, task.ID)
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
			return fmt.Errorf("invalid status '%s' for task '%s': must be one of: open, in_progress, closed", updateStatus, task.ID)
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

	// Link skills
	for _, skillName := range updateAddSkill {
		var skill models.Skill
		if err := database.Where("name = ?", skillName).First(&skill).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skill not found: %s\n", skillName)
			continue
		}
		// Check if already linked
		var existing models.TaskSkillLink
		if database.Where("task_id = ? AND skill_id = ?", task.ID, skill.ID).First(&existing).Error == nil {
			continue // Already linked
		}
		link := models.TaskSkillLink{TaskID: task.ID, SkillID: skill.ID}
		if err := database.Create(&link).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to link skill %s: %v\n", skillName, err)
			continue
		}
		models.RecordChange(database, task.ID, "skill_added", "", skillName, changedBy)
	}

	// Unlink skills
	for _, skillName := range updateRemoveSkill {
		var skill models.Skill
		if err := database.Where("name = ?", skillName).First(&skill).Error; err != nil {
			continue
		}
		if err := database.Where("task_id = ? AND skill_id = ?", task.ID, skill.ID).Delete(&models.TaskSkillLink{}).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to unlink skill %s: %v\n", skillName, err)
			continue
		}
		models.RecordChange(database, task.ID, "skill_removed", skillName, "", changedBy)
	}

	// Link agents
	for _, agentName := range updateAddAgent {
		var agent models.Agent
		if err := database.Where("name = ?", agentName).First(&agent).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Warning: agent not found: %s\n", agentName)
			continue
		}
		// Check if already linked
		var existing models.TaskAgentLink
		if database.Where("task_id = ? AND agent_id = ?", task.ID, agent.ID).First(&existing).Error == nil {
			continue // Already linked
		}
		link := models.TaskAgentLink{TaskID: task.ID, AgentID: agent.ID}
		if err := database.Create(&link).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to link agent %s: %v\n", agentName, err)
			continue
		}
		models.RecordChange(database, task.ID, "agent_added", "", agentName, changedBy)
	}

	// Unlink agents
	for _, agentName := range updateRemoveAgent {
		var agent models.Agent
		if err := database.Where("name = ?", agentName).First(&agent).Error; err != nil {
			continue
		}
		if err := database.Where("task_id = ? AND agent_id = ?", task.ID, agent.ID).Delete(&models.TaskAgentLink{}).Error; err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to unlink agent %s: %v\n", agentName, err)
			continue
		}
		models.RecordChange(database, task.ID, "agent_removed", agentName, "", changedBy)
	}

	if err := database.Save(&task).Error; err != nil {
		return fmt.Errorf("failed to update task '%s': database error: %w", task.ID, err)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "task": task})
	} else {
		fmt.Printf("Updated: %s\n", task.ID)
	}
	return nil
}
