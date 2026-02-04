package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	createPriority    int
	createType        string
	createLabels      []string
	createAssignee    string
	createDescription string
	createTemplate    string
	createParent      string
)

var createCmd = &cobra.Command{
	Use:   "create \"title\"",
	Short: "Create a new task",
	Args:  cobra.RangeArgs(0, 1),
	RunE:  runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().IntVarP(&createPriority, "priority", "p", -1, "Priority (0-4)")
	createCmd.Flags().StringVarP(&createType, "type", "t", "", "Type (task/bug/feature/epic)")
	createCmd.Flags().StringArrayVarP(&createLabels, "label", "l", nil, "Labels")
	createCmd.Flags().StringVarP(&createAssignee, "assignee", "a", "", "Assignee")
	createCmd.Flags().StringVarP(&createDescription, "description", "d", "", "Description")
	createCmd.Flags().StringVar(&createTemplate, "template", "", "Create from template")
	createCmd.Flags().StringVar(&createParent, "parent", "", "Parent task ID (creates subtask)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	var task *models.Task

	// If using a template, start with template values
	if createTemplate != "" {
		var template models.Template
		if err := db.GetDB().Where("name = ? OR id = ?", createTemplate, createTemplate).First(&template).Error; err != nil {
			return fmt.Errorf("template not found: %s", createTemplate)
		}
		task = template.ToTask()
	} else {
		task = &models.Task{
			Status:   models.StatusOpen,
			Priority: models.PriorityMedium,
			Type:     models.TypeTask,
		}
	}

	// Title from args (required unless template provides it)
	if len(args) > 0 {
		task.Title = args[0]
	}
	if task.Title == "" {
		return fmt.Errorf("title is required (provide as argument or use template with title)")
	}

	// Override with flags if provided
	if createPriority >= 0 {
		task.Priority = createPriority
	}
	if createType != "" {
		task.Type = createType
	}
	if createDescription != "" {
		task.Description = createDescription
	}
	if createAssignee != "" {
		task.Assignee = createAssignee
	}
	if len(createLabels) > 0 {
		task.Labels = createLabels
	}

	// Validate priority range
	if task.Priority < 0 || task.Priority > 4 {
		return fmt.Errorf("priority must be between 0 and 4")
	}

	// Validate type
	validTypes := map[string]bool{
		models.TypeTask:    true,
		models.TypeBug:     true,
		models.TypeFeature: true,
		models.TypeEpic:    true,
	}
	if !validTypes[task.Type] {
		return fmt.Errorf("invalid type: %s (must be task/bug/feature/epic)", task.Type)
	}

	database := db.GetDB()

	// Handle subtask creation
	if createParent != "" {
		var parent models.Task
		if err := database.First(&parent, "id = ?", createParent).Error; err != nil {
			return fmt.Errorf("parent task not found: %s", createParent)
		}
		if parent.IsClosed() {
			return fmt.Errorf("cannot create subtask of closed task")
		}

		// Count existing subtasks to generate next number
		var count int64
		database.Model(&models.Task{}).Where("parent_id = ?", createParent).Count(&count)
		task.ID = models.GenerateSubtaskID(createParent, int(count)+1)
		task.ParentID = createParent
	}

	if err := database.Create(task).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "task": task})
	} else {
		fmt.Printf("Created: %s - %s\n", task.ID, task.Title)
	}
	return nil
}
