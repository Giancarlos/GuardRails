package models

import (
	"time"
)

// Config stores key-value configuration for the project
type Config struct {
	Key       string    `gorm:"primaryKey;size:100" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for Config
func (Config) TableName() string {
	return "config"
}

// Common config keys
const (
	ConfigSchemaVersion = "schema_version"
	ConfigProjectName   = "project_name"
	ConfigInitializedAt = "initialized_at"
	ConfigIDPrefix      = "id_prefix"
	ConfigMode          = "mode"
)

// GitHub config keys
const (
	ConfigGitHubRepo        = "github_repo"         // owner/repo format
	ConfigGitHubIssuePrefix = "github_issue_prefix" // e.g., "[Coding Agent]"
	ConfigGitHubTokenSet    = "github_token_set"    // "true" if token stored in keyring
)

// Machine config keys
const (
	ConfigMachineName  = "machine_name"  // Friendly name for this machine
	ConfigMachineShare = "machine_share" // "true" to share name in sync markers
)

// Default values
const (
	DefaultGitHubIssuePrefix = "[Coding Agent]"
	KeyringServiceName       = "guardrails"
	KeyringGitHubTokenKey    = "github_token"
)

// Mode constants
const (
	ModeDefault     = "default"     // Standard mode - full integration
	ModeStealth     = "stealth"     // Local-only, not committed to repo
	ModeContributor = "contributor" // Separate tracking for contributors
)
