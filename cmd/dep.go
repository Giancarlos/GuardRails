package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var depType string

var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Dependency management",
}

var depAddCmd = &cobra.Command{
	Use:   "add <blocker-id> <blocked-id>",
	Short: "Add dependency: first task blocks the second",
	Long: `Add a dependency where the first task BLOCKS the second task.

Example: If Task B cannot start until Task A is done:
  gur dep add <task-A> <task-B>

This means:
  - Task A is the BLOCKER (must be done first)
  - Task B is BLOCKED (waiting on Task A)
  - Task B will NOT appear in 'gur ready' until Task A is closed`,
	Args: cobra.ExactArgs(2),
	RunE: runDepAdd,
}

var depRemoveCmd = &cobra.Command{
	Use:   "remove <blocker-id> <blocked-id>",
	Short: "Remove dependency between two tasks",
	Args:  cobra.ExactArgs(2),
	RunE:  runDepRemove,
}

var depListCmd = &cobra.Command{
	Use:   "list <id>",
	Short: "List dependencies for a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runDepList,
}

func init() {
	rootCmd.AddCommand(depCmd)
	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRemoveCmd)
	depCmd.AddCommand(depListCmd)

	depAddCmd.Flags().StringVarP(&depType, "type", "t", "blocks", "Type (blocks/related/parent-child)")
}

// wouldCreateCycle checks if adding blockerID -> blockedID would create a cycle
// by checking if blockedID can reach blockerID through existing dependencies
func wouldCreateCycle(database *gorm.DB, blockerID, blockedID string) bool {
	// BFS to check if blockedID can reach blockerID
	visited := make(map[string]bool)
	queue := []string{blockedID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		// Find all tasks that 'current' blocks (where current is the parent/blocker)
		var deps []models.Dependency
		database.Where("parent_id = ?", current).Find(&deps)

		for _, dep := range deps {
			if dep.ChildID == blockerID {
				// blockedID can reach blockerID - cycle detected
				return true
			}
			if !visited[dep.ChildID] {
				queue = append(queue, dep.ChildID)
			}
		}
	}
	return false
}

func runDepAdd(cmd *cobra.Command, args []string) error {
	blockerID, blockedID := args[0], args[1]
	database := db.GetDB()

	var blocker, blocked models.Task
	if err := database.Where("id = ?", blockerID).First(&blocker).Error; err != nil {
		return fmt.Errorf("blocker task not found: %s", blockerID)
	}
	if err := database.Where("id = ?", blockedID).First(&blocked).Error; err != nil {
		return fmt.Errorf("blocked task not found: %s", blockedID)
	}

	if blockerID == blockedID {
		return fmt.Errorf("task cannot block itself")
	}

	// Check for circular dependency
	if wouldCreateCycle(database, blockerID, blockedID) {
		return fmt.Errorf("circular dependency detected: %s already depends on %s", blockerID, blockedID)
	}

	dep := &models.Dependency{
		ChildID:  blockedID, // blocked task
		ParentID: blockerID, // blocker task
		Type:     depType,
	}

	if err := database.Create(dep).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "dependency": dep})
	} else {
		fmt.Printf("Added: %s blocks %s\n", blockerID, blockedID)
	}
	return nil
}

func runDepRemove(cmd *cobra.Command, args []string) error {
	blockerID, blockedID := args[0], args[1]
	database := db.GetDB()

	// Validate that both tasks exist
	var blocker, blocked models.Task
	if err := database.Where("id = ?", blockerID).First(&blocker).Error; err != nil {
		return fmt.Errorf("blocker task not found: %s", blockerID)
	}
	if err := database.Where("id = ?", blockedID).First(&blocked).Error; err != nil {
		return fmt.Errorf("blocked task not found: %s", blockedID)
	}

	result := database.Where("parent_id = ? AND child_id = ?", blockerID, blockedID).Delete(&models.Dependency{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("dependency not found between %s and %s", blockerID, blockedID)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true})
	} else {
		fmt.Println("Dependency removed")
	}
	return nil
}

func runDepList(cmd *cobra.Command, args []string) error {
	taskID := args[0]
	database := db.GetDB()

	var blockedBy, blocks []models.Dependency
	database.Where("child_id = ?", taskID).Find(&blockedBy)
	database.Where("parent_id = ?", taskID).Find(&blocks)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"blocked_by": blockedBy, "blocks": blocks})
		return nil
	}

	fmt.Printf("Dependencies for %s:\n", taskID)
	fmt.Printf("\nBlocked by (%d):\n", len(blockedBy))
	for _, d := range blockedBy {
		fmt.Printf("  - %s (%s)\n", d.ParentID, d.Type)
	}
	fmt.Printf("\nBlocks (%d):\n", len(blocks))
	for _, d := range blocks {
		fmt.Printf("  - %s (%s)\n", d.ChildID, d.Type)
	}
	return nil
}
