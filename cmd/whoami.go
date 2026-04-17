package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Giancarlos/guardrails/internal/db"
	"github.com/Giancarlos/guardrails/internal/models"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user and machine info",
	Long:  `Display information about the current user, machine, and GitHub configuration.`,
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Hash hostname
	h := sha256.Sum256([]byte(hostname))
	hostnameHash := hex.EncodeToString(h[:])[:8]

	// Get GitHub config if available
	repo, _ := db.GetConfig(models.ConfigGitHubRepo)
	username := ""
	if repo != "" {
		// Try to get GitHub username from a recent sync
		var link models.GitHubIssueLink
		if err := db.GetDB().Order("last_synced_at DESC").First(&link).Error; err == nil {
			username = link.SyncedBy
		}
	}

	// Get database path
	dbPath, _ := db.GetDefaultDBPath()

	// Get mode (defaults to "default" if unset, mirroring runConfigShow)
	mode := models.ModeDefault
	if modeConfig, err := db.GetConfig(models.ConfigMode); err == nil && modeConfig != "" {
		mode = modeConfig
	}

	if IsJSONOutput() {
		result := map[string]interface{}{
			"machine_hash": hostnameHash,
			"mode":         mode,
			"database":     dbPath,
		}
		if repo != "" {
			result["github_repo"] = repo
		}
		if username != "" {
			result["github_user"] = username
		}
		OutputJSON(result)
		return nil
	}

	if username != "" {
		fmt.Printf("GitHub:   @%s (%s)\n", username, repo)
	} else if repo != "" {
		fmt.Printf("GitHub:   %s\n", repo)
	} else {
		fmt.Println("GitHub:   (not configured — run 'gur config github')")
	}
	fmt.Printf("Machine:  %s\n", hostnameHash)
	fmt.Printf("Mode:     %s\n", mode)
	fmt.Printf("Database: %s\n", dbPath)

	return nil
}
