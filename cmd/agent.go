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

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage AI agents",
	Long: `Manage AI agents that can be linked to tasks.

Agents are defined in files like AGENTS.md, CLAUDE.md, or custom agent configurations.
When a task has linked agents, the agent working on it will be informed which agent to use or delegate to.`,
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered agents",
	RunE:  runAgentList,
}

var agentAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Register an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentAdd,
}

var agentRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Short:   "Unregister an agent",
	Aliases: []string{"rm"},
	Args:    cobra.ExactArgs(1),
	RunE:    runAgentRemove,
}

var agentShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show agent details",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentShow,
}

var agentScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Auto-discover agents from known locations",
	RunE:  runAgentScan,
}

var (
	agentPath         string
	agentSource       string
	agentDescription  string
	agentCapabilities string
)

func init() {
	rootCmd.AddCommand(agentCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentRemoveCmd)
	agentCmd.AddCommand(agentShowCmd)
	agentCmd.AddCommand(agentScanCmd)

	agentAddCmd.Flags().StringVar(&agentPath, "path", "", "Full path to agent file")
	agentAddCmd.Flags().StringVar(&agentSource, "source", models.SourceCustom, "Source (claude/cursor/windsurf/copilot/custom)")
	agentAddCmd.Flags().StringVarP(&agentDescription, "description", "d", "", "Agent description")
	agentAddCmd.Flags().StringVar(&agentCapabilities, "capabilities", "", "Agent capabilities")
}

func runAgentList(cmd *cobra.Command, args []string) error {
	var agents []models.Agent
	if err := db.GetDB().Find(&agents).Error; err != nil {
		return err
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"count": len(agents), "agents": agents})
		return nil
	}

	if len(agents) == 0 {
		fmt.Println("No agents registered. Run 'gur agent scan' to auto-discover or 'gur agent add' to register manually.")
		return nil
	}

	fmt.Printf("Registered Agents (%d):\n", len(agents))
	for _, a := range agents {
		fmt.Printf("  [%d] %s", a.ID, a.Name)
		if a.Source != models.SourceCustom {
			fmt.Printf(" (%s)", a.Source)
		}
		if a.Description != "" {
			desc := a.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Printf(" - %s", desc)
		}
		fmt.Println()
	}
	return nil
}

func runAgentAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Check if already exists
	var existing models.Agent
	if err := db.GetDB().Where("name = ?", name).First(&existing).Error; err == nil {
		return fmt.Errorf("cannot add agent: agent '%s' already exists (use 'gur agent show %s' to view it)", name, name)
	}

	agent := models.Agent{
		Name:         name,
		Path:         agentPath,
		Source:       agentSource,
		Description:  agentDescription,
		Capabilities: agentCapabilities,
	}

	if err := db.GetDB().Create(&agent).Error; err != nil {
		return fmt.Errorf("failed to register agent '%s': database error: %w", name, err)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "agent": agent})
	} else {
		fmt.Printf("Registered agent: %s\n", name)
	}
	return nil
}

func runAgentRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	var agent models.Agent
	if err := db.GetDB().Where("name = ?", name).First(&agent).Error; err != nil {
		return fmt.Errorf("cannot remove agent: agent '%s' not found (use 'gur agent list' to see registered agents)", name)
	}

	// Remove task links first
	if err := db.GetDB().Where("agent_id = ?", agent.ID).Delete(&models.TaskAgentLink{}).Error; err != nil {
		return fmt.Errorf("failed to remove agent '%s': could not delete task links: %w", name, err)
	}

	if err := db.GetDB().Delete(&agent).Error; err != nil {
		return fmt.Errorf("failed to remove agent '%s': database error: %w", name, err)
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "message": fmt.Sprintf("Removed agent: %s", name)})
	} else {
		fmt.Printf("Removed agent: %s\n", name)
	}
	return nil
}

func runAgentShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	var agent models.Agent
	if err := db.GetDB().Where("name = ? OR id = ?", name, name).First(&agent).Error; err != nil {
		return fmt.Errorf("agent '%s' not found (use 'gur agent list' to see registered agents, or 'gur agent scan' to auto-discover)", name)
	}

	// Get linked tasks
	var links []models.TaskAgentLink
	db.GetDB().Where("agent_id = ?", agent.ID).Find(&links)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"agent": agent, "linked_tasks": len(links)})
		return nil
	}

	fmt.Printf("ID:           %d\n", agent.ID)
	fmt.Printf("Name:         %s\n", agent.Name)
	fmt.Printf("Source:       %s\n", agent.Source)
	if agent.Path != "" {
		fmt.Printf("Path:         %s\n", agent.Path)
	}
	if agent.Description != "" {
		fmt.Printf("Description:  %s\n", agent.Description)
	}
	if agent.Capabilities != "" {
		fmt.Printf("Capabilities: %s\n", agent.Capabilities)
	}
	fmt.Printf("Linked to:    %d task(s)\n", len(links))

	return nil
}

func runAgentScan(cmd *cobra.Command, args []string) error {
	homeDir, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	discovered := 0
	skipped := 0

	// Scan Claude agents directory
	claudeAgentDirs := []string{
		filepath.Join(homeDir, ".claude", "agents"),
		filepath.Join(cwd, ".claude", "agents"),
	}

	for _, dir := range claudeAgentDirs {
		agents, err := scanAgentDirectory(dir, models.SourceClaude)
		if err != nil {
			continue
		}
		for _, a := range agents {
			added, err := registerAgentIfNew(a)
			if err != nil {
				if !IsJSONOutput() {
					fmt.Printf("  Error: %s - %v\n", a.Name, err)
				}
			} else if added {
				discovered++
				if !IsJSONOutput() {
					fmt.Printf("  Found: %s (%s)\n", a.Name, a.Source)
				}
			} else {
				skipped++
			}
		}
	}

	// Scan for standard agent files in project root
	standardAgentFiles := []struct {
		name   string
		source string
	}{
		{"AGENTS.md", models.SourceCustom},
		{"CLAUDE.md", models.SourceClaude},
		{"GEMINI.md", models.SourceCustom},
		{".cursorrules", models.SourceCursor},
		{".windsurfrules", models.SourceWindsurf},
	}

	for _, af := range standardAgentFiles {
		agentPath := filepath.Join(cwd, af.name)
		if _, err := os.Stat(agentPath); os.IsNotExist(err) {
			continue
		}

		agent := models.Agent{
			Name:        strings.TrimSuffix(strings.TrimPrefix(af.name, "."), ".md"),
			Path:        agentPath,
			Source:      af.source,
			Description: extractAgentDescription(agentPath),
		}

		added, err := registerAgentIfNew(agent)
		if err != nil {
			if !IsJSONOutput() {
				fmt.Printf("  Error: %s - %v\n", agent.Name, err)
			}
		} else if added {
			discovered++
			if !IsJSONOutput() {
				fmt.Printf("  Found: %s (%s)\n", agent.Name, agent.Source)
			}
		} else {
			skipped++
		}
	}

	// Register built-in Claude Code agents
	builtInAgents := []models.Agent{
		{Name: "Explore", Source: models.SourceClaude, Description: "Fast agent for exploring codebases", Capabilities: "Glob, Grep, Read, WebFetch, WebSearch"},
		{Name: "Plan", Source: models.SourceClaude, Description: "Software architect for designing implementation plans", Capabilities: "All read tools, no edit/write"},
		{Name: "Bash", Source: models.SourceClaude, Description: "Command execution specialist", Capabilities: "Bash commands, git operations"},
	}

	for _, a := range builtInAgents {
		added, err := registerAgentIfNew(a)
		if err != nil {
			if !IsJSONOutput() {
				fmt.Printf("  Error: %s - %v\n", a.Name, err)
			}
		} else if added {
			discovered++
			if !IsJSONOutput() {
				fmt.Printf("  Found: %s (built-in)\n", a.Name)
			}
		} else {
			skipped++
		}
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "discovered": discovered, "skipped": skipped})
	} else {
		fmt.Printf("\nDiscovered %d new agent(s), %d already registered\n", discovered, skipped)
	}
	return nil
}

func scanAgentDirectory(dir string, source string) ([]models.Agent, error) {
	var agents []models.Agent

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}

		agentPath := filepath.Join(dir, name)
		agentName := strings.TrimSuffix(name, ".md")

		agent := models.Agent{
			Name:        agentName,
			Path:        agentPath,
			Source:      source,
			Description: extractAgentDescription(agentPath),
		}
		agents = append(agents, agent)
	}

	return agents, nil
}

func extractAgentDescription(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Try to find a description in frontmatter or first paragraph
	inFrontmatter := false
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				inFrontmatter = false
				continue
			}
		}

		if inFrontmatter && strings.HasPrefix(line, "description:") {
			desc := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			return strings.Trim(desc, "\"'")
		}

		// If no frontmatter, use first non-empty, non-heading line
		if !inFrontmatter && lineCount <= 10 {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "---") {
				if len(line) > 100 {
					line = line[:97] + "..."
				}
				return line
			}
		}
	}

	return ""
}

func registerAgentIfNew(agent models.Agent) (bool, error) {
	var existing models.Agent
	if err := db.GetDB().Where("name = ?", agent.Name).First(&existing).Error; err == nil {
		return false, nil // Already exists
	}

	if err := db.GetDB().Create(&agent).Error; err != nil {
		return false, err
	}
	return true, nil
}
