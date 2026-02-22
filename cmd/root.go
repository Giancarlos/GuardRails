package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Giancarlos/guardrails/internal/db"
	"github.com/Giancarlos/guardrails/internal/output"
)

var (
	Version       = "0.1.0"
	jsonOutput    bool
	compactOutput bool
	verboseOutput bool
)

// commandsExemptFromDB lists commands that don't require database initialization
var commandsExemptFromDB = map[string]bool{
	"init":       true,
	"version":    true,
	"help":       true,
	"completion": true,
}

var rootCmd = &cobra.Command{
	Use:   "gur",
	Short: "GuardRails - A SQLite-based task manager for AI agents",
	Long: `GuardRails (gur) is a command-line task management tool for AI agents.

QUICK START:
  gur init                          # Initialize in current directory
  gur create "Fix bug" -p 0         # Create P0 (critical) task
  gur list                          # List all tasks
  gur ready                         # Show tasks ready to work on
  gur update <id> -s in_progress    # Start working on task
  gur close <id> -r "Done"          # Close with reason

PRIORITIES: 0=Critical, 1=High, 2=Medium (default), 3=Low, 4=Lowest
TYPES: task (default), bug, feature, epic
STATUSES: open, in_progress, closed

TASK IDS: Auto-generated like "gur-a1b2c3d4"

DEPENDENCIES:
  gur dep add <blocker> <blocked>   # First task blocks the second
  gur ready                         # Shows only unblocked tasks

TEST CASES:
  gur test create "Login works" -c auth -t e2e
  gur test list                     # List all tests
  gur test run <id> passed/failed   # Record test result
  gur test link <test-id> <task-id> # Link test to task (required to close)

TEST TYPES: unit, integration, e2e, manual, smoke, regression

WORKFLOW: Tasks with linked tests cannot be closed until tests pass.

JSON OUTPUT: Add --json flag to any command for machine-readable output.
COMPACT OUTPUT: Enabled by default. Use --verbose or GUR_COMPACT=0 for full output.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		output.SetCompact(IsCompactOutput())
		if commandsExemptFromDB[cmd.Name()] {
			return nil
		}
		return db.EnsureInitialized()
	},
}

func Execute() {
	defer db.CloseDB()

	if err := rootCmd.Execute(); err != nil {
		if jsonOutput {
			OutputJSON(map[string]interface{}{"error": true, "message": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&compactOutput, "compact", "c", true, "Token-optimized compact output (default, use --compact=false to disable)")
	rootCmd.PersistentFlags().BoolVar(&verboseOutput, "verbose", false, "Use verbose text output instead of compact")
	rootCmd.Version = Version
}

// IsCompactOutput returns whether compact mode is active (default: true)
func IsCompactOutput() bool {
	if os.Getenv("GUR_COMPACT") == "0" || verboseOutput {
		return false
	}
	return compactOutput
}

func OutputJSON(data interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	encoder.Encode(data)
}

func IsJSONOutput() bool {
	return jsonOutput
}
