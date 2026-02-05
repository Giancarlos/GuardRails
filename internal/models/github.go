package models

import (
	"time"
)

// Sync direction constants
const (
	SyncDirectionPush = "push"
	SyncDirectionPull = "pull"
	SyncDirectionBoth = "both"
)

// GitHubIssueLink tracks the mapping between gur tasks and GitHub issues
type GitHubIssueLink struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	TaskID          string     `gorm:"size:30;uniqueIndex;not null" json:"task_id"`
	IssueNumber     int        `gorm:"not null;index" json:"issue_number"`
	IssueURL        string     `gorm:"size:500" json:"issue_url"`
	Repository      string     `gorm:"size:200;not null;index" json:"repository"` // owner/repo format
	LastSyncedAt    time.Time  `json:"last_synced_at"`
	RemoteUpdatedAt *time.Time `json:"remote_updated_at,omitempty"` // GitHub issue updated_at
	SyncDirection   string     `gorm:"size:10;default:push" json:"sync_direction"`
	SyncedBy        string     `gorm:"size:100" json:"synced_by,omitempty"`      // username who synced
	SyncedMachine   string     `gorm:"size:100" json:"synced_machine,omitempty"` // machine hostname
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for GitHubIssueLink
func (GitHubIssueLink) TableName() string {
	return "github_issue_links"
}
