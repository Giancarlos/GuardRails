package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var (
	forceInit       bool
	stealthMode     bool
	contributorMode bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize GuardRails in the current directory",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Force reinitialize")
	initCmd.Flags().BoolVar(&stealthMode, "stealth", false, "Initialize in stealth mode (local-only, add to .gitignore)")
	initCmd.Flags().BoolVar(&contributorMode, "contributor", false, "Initialize in contributor mode (separate tracking)")
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	guardrailsDir := filepath.Join(cwd, db.GuardrailsDir)
	dbPath := filepath.Join(guardrailsDir, db.DBFileName)

	// Check if already initialized
	if info, err := os.Stat(guardrailsDir); err == nil && info.IsDir() {
		if !forceInit {
			return fmt.Errorf("already initialized. Use --force to reinitialize")
		}
		// Remove existing guardrails directory
		if err := os.RemoveAll(guardrailsDir); err != nil {
			return fmt.Errorf("failed to remove existing guardrails directory: %w", err)
		}
	}

	// Create .guardrails directory
	if err := os.MkdirAll(guardrailsDir, 0755); err != nil {
		return fmt.Errorf("failed to create guardrails directory: %w", err)
	}

	database, err := db.InitDB(dbPath)
	if err != nil {
		return err
	}

	if err := database.Create(&models.Config{Key: models.ConfigSchemaVersion, Value: db.SchemaVersion}).Error; err != nil {
		return fmt.Errorf("failed to save schema version: %w", err)
	}
	if err := database.Create(&models.Config{Key: models.ConfigInitializedAt, Value: time.Now().Format(time.RFC3339)}).Error; err != nil {
		return fmt.Errorf("failed to save initialization time: %w", err)
	}

	// Determine and save mode
	mode := models.ModeDefault
	if stealthMode {
		mode = models.ModeStealth
	} else if contributorMode {
		mode = models.ModeContributor
	}
	if err := database.Create(&models.Config{Key: models.ConfigMode, Value: mode}).Error; err != nil {
		return fmt.Errorf("failed to save mode: %w", err)
	}

	// In stealth mode, add .guardrails to .gitignore
	if stealthMode {
		if err := addToGitignore(cwd, db.GuardrailsDir); err != nil {
			// Non-fatal, just warn
			fmt.Fprintf(os.Stderr, "Warning: could not add to .gitignore: %v\n", err)
		}
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "path": guardrailsDir, "mode": mode})
		return nil
	}

	modeStr := ""
	if mode != models.ModeDefault {
		modeStr = fmt.Sprintf(" (mode: %s)", mode)
	}
	fmt.Printf("GuardRails initialized in %s/%s\n", db.GuardrailsDir, modeStr)

	// Detect git repo and offer helpful next steps
	isGitRepo := false
	if _, err := os.Stat(filepath.Join(cwd, ".git")); err == nil {
		isGitRepo = true
	}

	fmt.Println("\nNext steps:")
	fmt.Println("  gur create \"My first task\"     Create a task")
	fmt.Println("  gur list                        List all tasks")
	if isGitRepo {
		fmt.Println("  gur config github               Setup GitHub sync (optional)")
	}

	return nil
}

func addToGitignore(dir, entry string) error {
	gitignorePath := filepath.Join(dir, ".gitignore")

	// Read existing content
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Check if already present
	lines := string(content)
	if lines != "" && (lines == entry ||
		len(lines) >= len(entry)+1 && (lines[:len(entry)+1] == entry+"\n" ||
			contains(lines, "\n"+entry+"\n") ||
			(len(lines) >= len(entry) && lines[len(lines)-len(entry):] == entry))) {
		return nil // Already in gitignore
	}

	// Append entry
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Add newline if file doesn't end with one
	if len(content) > 0 && content[len(content)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(entry + "\n")
	return err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
