package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage AI skills",
	Long: `Manage AI skills that can be linked to tasks.

Skills are SKILL.md files that provide domain-specific instructions for AI agents.
When a task has linked skills, the agent working on it will be informed which skills to use.`,
}

var skillListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered skills",
	RunE:  runSkillList,
}

var skillAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Register a skill",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillAdd,
}

var skillRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Unregister a skill",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runSkillRemove,
}

var skillShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show skill details",
	Args:  cobra.ExactArgs(1),
	RunE:  runSkillShow,
}

var skillScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Auto-discover skills from known locations",
	RunE:  runSkillScan,
}

var (
	skillPath        string
	skillSource      string
	skillDescription string
)

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillListCmd)
	skillCmd.AddCommand(skillAddCmd)
	skillCmd.AddCommand(skillRemoveCmd)
	skillCmd.AddCommand(skillShowCmd)
	skillCmd.AddCommand(skillScanCmd)

	skillAddCmd.Flags().StringVar(&skillPath, "path", "", "Full path to skill file")
	skillAddCmd.Flags().StringVar(&skillSource, "source", models.SourceCustom, "Source (claude/cursor/windsurf/copilot/custom)")
	skillAddCmd.Flags().StringVarP(&skillDescription, "description", "d", "", "Skill description")
}

func runSkillList(cmd *cobra.Command, args []string) error {
	var skills []models.Skill
	if err := db.GetDB().Find(&skills).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"count": len(skills), "skills": skills})
		return nil
	}

	if len(skills) == 0 {
		fmt.Println("No skills registered. Run 'gur skill scan' to auto-discover or 'gur skill add' to register manually.")
		return nil
	}

	fmt.Printf("Registered Skills (%d):\n", len(skills))
	for _, s := range skills {
		fmt.Printf("  [%d] %s", s.ID, s.Name)
		if s.Source != models.SourceCustom {
			fmt.Printf(" (%s)", s.Source)
		}
		if s.Description != "" {
			desc := s.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Printf(" - %s", desc)
		}
		fmt.Println()
	}
	return nil
}

func runSkillAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Check if already exists
	var existing models.Skill
	if err := db.GetDB().Where("name = ?", name).First(&existing).Error; err == nil {
		return fmt.Errorf("skill '%s' already exists", name)
	}

	skill := models.Skill{
		Name:        name,
		Path:        skillPath,
		Source:      skillSource,
		Description: skillDescription,
	}

	// If path provided, try to read description from SKILL.md
	if skillPath != "" && skillDescription == "" {
		if desc := extractSkillDescription(skillPath); desc != "" {
			skill.Description = desc
		}
	}

	if err := db.GetDB().Create(&skill).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "skill": skill})
	} else {
		fmt.Printf("Registered skill: %s\n", name)
	}
	return nil
}

func runSkillRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	var skill models.Skill
	if err := db.GetDB().Where("name = ?", name).First(&skill).Error; err != nil {
		return fmt.Errorf("skill not found: %s", name)
	}

	// Remove task links first
	db.GetDB().Where("skill_id = ?", skill.ID).Delete(&models.TaskSkillLink{})

	if err := db.GetDB().Delete(&skill).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "message": fmt.Sprintf("Removed skill: %s", name)})
	} else {
		fmt.Printf("Removed skill: %s\n", name)
	}
	return nil
}

func runSkillShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	var skill models.Skill
	if err := db.GetDB().Where("name = ? OR id = ?", name, name).First(&skill).Error; err != nil {
		return fmt.Errorf("skill not found: %s", name)
	}

	// Get linked tasks
	var links []models.TaskSkillLink
	db.GetDB().Where("skill_id = ?", skill.ID).Find(&links)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"skill": skill, "linked_tasks": len(links)})
		return nil
	}

	fmt.Printf("ID:          %d\n", skill.ID)
	fmt.Printf("Name:        %s\n", skill.Name)
	fmt.Printf("Source:      %s\n", skill.Source)
	if skill.Path != "" {
		fmt.Printf("Path:        %s\n", skill.Path)
	}
	if skill.Description != "" {
		fmt.Printf("Description: %s\n", skill.Description)
	}
	fmt.Printf("Linked to:   %d task(s)\n", len(links))

	return nil
}

func runSkillScan(cmd *cobra.Command, args []string) error {
	homeDir, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	discovered := 0
	skipped := 0

	// Scan Claude skills
	claudeSkillDirs := []string{
		filepath.Join(homeDir, ".claude", "skills"),
		filepath.Join(cwd, ".claude", "skills"),
	}

	for _, dir := range claudeSkillDirs {
		skills, err := scanSkillDirectory(dir, models.SourceClaude)
		if err != nil {
			continue
		}
		for _, s := range skills {
			added, err := registerSkillIfNew(s)
			if err != nil {
				if !IsJSONOutput() {
					fmt.Printf("  Error: %s - %v\n", s.Name, err)
				}
			} else if added {
				discovered++
				if !IsJSONOutput() {
					fmt.Printf("  Found: %s (%s)\n", s.Name, s.Source)
				}
			} else {
				skipped++
			}
		}
	}

	// Scan Cursor rules
	cursorRuleDirs := []string{
		filepath.Join(homeDir, ".cursor", "rules"),
		filepath.Join(cwd, ".cursor", "rules"),
	}

	for _, dir := range cursorRuleDirs {
		skills, err := scanCursorRules(dir)
		if err != nil {
			continue
		}
		for _, s := range skills {
			added, err := registerSkillIfNew(s)
			if err != nil {
				if !IsJSONOutput() {
					fmt.Printf("  Error: %s - %v\n", s.Name, err)
				}
			} else if added {
				discovered++
				if !IsJSONOutput() {
					fmt.Printf("  Found: %s (%s)\n", s.Name, s.Source)
				}
			} else {
				skipped++
			}
		}
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "discovered": discovered, "skipped": skipped})
	} else {
		fmt.Printf("\nDiscovered %d new skill(s), %d already registered\n", discovered, skipped)
	}
	return nil
}

func scanSkillDirectory(dir string, source string) ([]models.Skill, error) {
	var skills []models.Skill

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			continue
		}

		skill := models.Skill{
			Name:        entry.Name(),
			Path:        skillPath,
			Source:      source,
			Description: extractSkillDescription(skillPath),
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

func scanCursorRules(dir string) ([]models.Skill, error) {
	var skills []models.Skill

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".mdc") && !strings.HasSuffix(name, ".md") {
			continue
		}

		skillPath := filepath.Join(dir, name)
		skillName := strings.TrimSuffix(strings.TrimSuffix(name, ".mdc"), ".md")

		skill := models.Skill{
			Name:        skillName,
			Path:        skillPath,
			Source:      models.SourceCursor,
			Description: extractSkillDescription(skillPath),
		}
		skills = append(skills, skill)
	}

	return skills, nil
}

func extractSkillDescription(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	foundDescription := ""

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				break // End of frontmatter
			}
		}

		if inFrontmatter && strings.HasPrefix(line, "description:") {
			foundDescription = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			foundDescription = strings.Trim(foundDescription, "\"'")
			break
		}
	}

	return foundDescription
}

func registerSkillIfNew(skill models.Skill) (bool, error) {
	var existing models.Skill
	if err := db.GetDB().Where("name = ?", skill.Name).First(&existing).Error; err == nil {
		return false, nil // Already exists
	}

	if err := db.GetDB().Create(&skill).Error; err != nil {
		return false, err
	}
	return true, nil
}
