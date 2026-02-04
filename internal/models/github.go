package models

import (
	"time"
)

// GitHubIssueLink tracks the mapping between gur tasks and GitHub issues
type GitHubIssueLink struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TaskID       string    `gorm:"size:30;uniqueIndex;not null" json:"task_id"`
	IssueNumber  int       `gorm:"not null" json:"issue_number"`
	IssueURL     string    `gorm:"size:500" json:"issue_url"`
	Repository   string    `gorm:"size:200;not null" json:"repository"` // owner/repo format
	LastSyncedAt time.Time `json:"last_synced_at"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for GitHubIssueLink
func (GitHubIssueLink) TableName() string {
	return "github_issue_links"
}
