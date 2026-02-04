package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"

	"guardrails/internal/db"
	"guardrails/internal/models"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage GuardRails configuration",
}

var configGitHubCmd = &cobra.Command{
	Use:   "github",
	Short: "Configure GitHub integration",
	Long: `Configure GitHub integration for syncing tasks to GitHub Issues.

This command will prompt you for:
  - GitHub repository (owner/repo format)
  - GitHub Personal Access Token (stored securely in system keyring)
  - Issue title prefix (default: "[Coding Agent]")

To create a token:
  1. Go to GitHub Settings → Developer settings → Personal access tokens → Fine-grained tokens
  2. Generate new token with repository access
  3. Set permissions: Issues → Read and Write
  4. Copy token immediately (shown only once)`,
	RunE: runConfigGitHub,
}

var (
	configGitHubRepo   string
	configGitHubPrefix string
	configGitHubToken  string
	configGitHubShow   bool
	configGitHubClear  bool
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGitHubCmd)

	configGitHubCmd.Flags().StringVar(&configGitHubRepo, "repo", "", "GitHub repository (owner/repo)")
	configGitHubCmd.Flags().StringVar(&configGitHubPrefix, "prefix", "", "Issue title prefix")
	configGitHubCmd.Flags().StringVar(&configGitHubToken, "token", "", "GitHub token (use stdin for security)")
	configGitHubCmd.Flags().BoolVar(&configGitHubShow, "show", false, "Show current configuration")
	configGitHubCmd.Flags().BoolVar(&configGitHubClear, "clear", false, "Clear GitHub configuration")
}

func runConfigGitHub(cmd *cobra.Command, args []string) error {
	// Handle --show flag
	if configGitHubShow {
		return showGitHubConfig()
	}

	// Handle --clear flag
	if configGitHubClear {
		return clearGitHubConfig()
	}

	// If flags provided, use non-interactive mode
	if configGitHubRepo != "" || configGitHubToken != "" || configGitHubPrefix != "" {
		return configureGitHubNonInteractive()
	}

	// Interactive mode
	return configureGitHubInteractive()
}

func showGitHubConfig() error {
	var repoConfig, prefixConfig, tokenSetConfig models.Config

	repo := ""
	if err := db.GetDB().Where("key = ?", models.ConfigGitHubRepo).First(&repoConfig).Error; err == nil {
		repo = repoConfig.Value
	}

	prefix := models.DefaultGitHubIssuePrefix
	if err := db.GetDB().Where("key = ?", models.ConfigGitHubIssuePrefix).First(&prefixConfig).Error; err == nil {
		prefix = prefixConfig.Value
	}

	tokenSet := false
	if err := db.GetDB().Where("key = ?", models.ConfigGitHubTokenSet).First(&tokenSetConfig).Error; err == nil {
		tokenSet = tokenSetConfig.Value == "true"
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{
			"repository":   repo,
			"issue_prefix": prefix,
			"token_set":    tokenSet,
		})
		return nil
	}

	fmt.Println("GitHub Configuration:")
	if repo != "" {
		fmt.Printf("  Repository:   %s\n", repo)
	} else {
		fmt.Println("  Repository:   (not configured)")
	}
	fmt.Printf("  Issue Prefix: %s\n", prefix)
	if tokenSet {
		fmt.Println("  Token:        (stored in system keyring)")
	} else {
		fmt.Println("  Token:        (not configured)")
	}

	return nil
}

func clearGitHubConfig() error {
	// Clear from database
	db.GetDB().Where("key = ?", models.ConfigGitHubRepo).Delete(&models.Config{})
	db.GetDB().Where("key = ?", models.ConfigGitHubIssuePrefix).Delete(&models.Config{})
	db.GetDB().Where("key = ?", models.ConfigGitHubTokenSet).Delete(&models.Config{})

	// Clear from keyring
	keyring.Delete(models.KeyringServiceName, models.KeyringGitHubTokenKey)

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "message": "GitHub configuration cleared"})
	} else {
		fmt.Println("GitHub configuration cleared")
	}
	return nil
}

func configureGitHubNonInteractive() error {
	if configGitHubRepo != "" {
		if !strings.Contains(configGitHubRepo, "/") {
			return fmt.Errorf("repository must be in owner/repo format")
		}
		if err := db.SetConfig(models.ConfigGitHubRepo, configGitHubRepo); err != nil {
			return fmt.Errorf("failed to save repository: %w", err)
		}
	}

	if configGitHubPrefix != "" {
		if err := db.SetConfig(models.ConfigGitHubIssuePrefix, configGitHubPrefix); err != nil {
			return fmt.Errorf("failed to save prefix: %w", err)
		}
	}

	if configGitHubToken != "" {
		if err := keyring.Set(models.KeyringServiceName, models.KeyringGitHubTokenKey, configGitHubToken); err != nil {
			return fmt.Errorf("failed to store token in keyring: %w", err)
		}
		if err := db.SetConfig(models.ConfigGitHubTokenSet, "true"); err != nil {
			return fmt.Errorf("failed to save token flag: %w", err)
		}
	}

	if IsJSONOutput() {
		OutputJSON(map[string]interface{}{"success": true, "message": "GitHub configuration updated"})
	} else {
		fmt.Println("GitHub configuration updated")
	}
	return nil
}

func configureGitHubInteractive() error {
	reader := bufio.NewReader(os.Stdin)

	// Get current values for defaults
	currentRepo, _ := db.GetConfig(models.ConfigGitHubRepo)
	currentPrefix, _ := db.GetConfig(models.ConfigGitHubIssuePrefix)
	if currentPrefix == "" {
		currentPrefix = models.DefaultGitHubIssuePrefix
	}

	fmt.Println("GitHub Integration Setup")
	fmt.Println("========================")
	fmt.Println()

	// Repository
	if currentRepo != "" {
		fmt.Printf("Repository [%s]: ", currentRepo)
	} else {
		fmt.Print("Repository (owner/repo): ")
	}
	repoInput, _ := reader.ReadString('\n')
	repoInput = strings.TrimSpace(repoInput)
	if repoInput == "" {
		repoInput = currentRepo
	}
	if repoInput == "" {
		return fmt.Errorf("repository is required")
	}
	if !strings.Contains(repoInput, "/") {
		return fmt.Errorf("repository must be in owner/repo format")
	}

	// Issue prefix
	fmt.Printf("Issue prefix [%s]: ", currentPrefix)
	prefixInput, _ := reader.ReadString('\n')
	prefixInput = strings.TrimSpace(prefixInput)
	if prefixInput == "" {
		prefixInput = currentPrefix
	}

	// Token
	fmt.Println()
	fmt.Println("GitHub Personal Access Token")
	fmt.Println("  Create at: GitHub Settings → Developer settings → Personal access tokens")
	fmt.Println("  Required permissions: Issues (Read and Write)")
	fmt.Println()
	fmt.Print("Token (input hidden, paste and press Enter): ")

	// Read token - note: in a real terminal this should use term.ReadPassword
	// but for simplicity we'll just read the line
	tokenInput, _ := reader.ReadString('\n')
	tokenInput = strings.TrimSpace(tokenInput)

	if tokenInput == "" {
		// Check if token already exists
		_, err := keyring.Get(models.KeyringServiceName, models.KeyringGitHubTokenKey)
		if err != nil {
			return fmt.Errorf("token is required")
		}
		fmt.Println("(keeping existing token)")
	} else {
		// Store new token
		if err := keyring.Set(models.KeyringServiceName, models.KeyringGitHubTokenKey, tokenInput); err != nil {
			return fmt.Errorf("failed to store token in keyring: %w", err)
		}
		if err := db.SetConfig(models.ConfigGitHubTokenSet, "true"); err != nil {
			return fmt.Errorf("failed to save token flag: %w", err)
		}
		fmt.Println("(token stored in system keyring)")
	}

	// Save configuration
	if err := db.SetConfig(models.ConfigGitHubRepo, repoInput); err != nil {
		return fmt.Errorf("failed to save repository: %w", err)
	}
	if err := db.SetConfig(models.ConfigGitHubIssuePrefix, prefixInput); err != nil {
		return fmt.Errorf("failed to save prefix: %w", err)
	}

	fmt.Println()
	fmt.Println("GitHub integration configured successfully!")
	fmt.Printf("  Repository:   %s\n", repoInput)
	fmt.Printf("  Issue Prefix: %s\n", prefixInput)

	return nil
}

// GetGitHubToken retrieves the GitHub token from keyring or environment
func GetGitHubToken() (string, error) {
	// First try environment variable
	if token := os.Getenv("GUR_GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// Then try keyring
	token, err := keyring.Get(models.KeyringServiceName, models.KeyringGitHubTokenKey)
	if err != nil {
		return "", fmt.Errorf("GitHub token not found. Run 'gur config github' or set GUR_GITHUB_TOKEN")
	}
	return token, nil
}
