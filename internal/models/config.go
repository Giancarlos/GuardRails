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

// Mode constants
const (
	ModeDefault     = "default"     // Standard mode - full integration
	ModeStealth     = "stealth"     // Local-only, not committed to repo
	ModeContributor = "contributor" // Separate tracking for contributors
)
