package models

import (
	"time"

	"gorm.io/gorm"
)

// Skill source constants
const (
	SourceClaude   = "claude"
	SourceCursor   = "cursor"
	SourceWindsurf = "windsurf"
	SourceCopilot  = "copilot"
	SourceCustom   = "custom"
)

// Skill represents a registered AI skill (SKILL.md files)
type Skill struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Path        string         `gorm:"size:500" json:"path,omitempty"`
	Description string         `gorm:"type:text" json:"description,omitempty"`
	Source      string         `gorm:"size:50;default:custom" json:"source"`
	Metadata    string         `gorm:"type:text" json:"metadata,omitempty"` // JSON for additional frontmatter
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Skill
func (Skill) TableName() string {
	return "skills"
}

// Agent represents a registered AI agent (AGENT.md, CLAUDE.md, etc.)
type Agent struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:100;uniqueIndex;not null" json:"name"`
	Path         string         `gorm:"size:500" json:"path,omitempty"`
	Description  string         `gorm:"type:text" json:"description,omitempty"`
	Source       string         `gorm:"size:50;default:custom" json:"source"`
	Capabilities string         `gorm:"type:text" json:"capabilities,omitempty"`
	Metadata     string         `gorm:"type:text" json:"metadata,omitempty"` // JSON for additional data
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Agent
func (Agent) TableName() string {
	return "agents"
}

// TaskSkillLink represents a many-to-many relationship between tasks and skills
type TaskSkillLink struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    string    `gorm:"size:30;index;not null" json:"task_id"`
	SkillID   uint      `gorm:"index;not null" json:"skill_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Skill Skill `gorm:"foreignKey:SkillID" json:"skill,omitempty"`
}

// TableName specifies the table name for TaskSkillLink
func (TaskSkillLink) TableName() string {
	return "task_skill_links"
}

// TaskAgentLink represents a many-to-many relationship between tasks and agents
type TaskAgentLink struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    string    `gorm:"size:30;index;not null" json:"task_id"`
	AgentID   uint      `gorm:"index;not null" json:"agent_id"`
	IsPrimary bool      `gorm:"default:false" json:"is_primary"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// Relationships
	Agent Agent `gorm:"foreignKey:AgentID" json:"agent,omitempty"`
}

// TableName specifies the table name for TaskAgentLink
func (TaskAgentLink) TableName() string {
	return "task_agent_links"
}

// SkillDiscoveryPaths returns the standard paths to search for skills
func SkillDiscoveryPaths() []string {
	return []string{
		"~/.claude/skills/*/SKILL.md",
		".claude/skills/*/SKILL.md",
		"~/.cursor/rules/*.mdc",
		".cursor/rules/*.mdc",
		"~/.copilot/skills/*/SKILL.md",
		".github/skills/*/SKILL.md",
	}
}

// AgentDiscoveryPaths returns the standard paths to search for agents
func AgentDiscoveryPaths() []string {
	return []string{
		"~/.claude/agents/*.md",
		".claude/agents/*.md",
		"AGENTS.md",
		"CLAUDE.md",
		"GEMINI.md",
		".cursorrules",
		".windsurfrules",
	}
}
