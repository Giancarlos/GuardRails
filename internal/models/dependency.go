package models

import (
	"time"

	"gorm.io/gorm"
)

// Dependency type constants
const (
	DepTypeBlocks      = "blocks"
	DepTypeRelated     = "related"
	DepTypeParentChild = "parent-child"
)

// Dependency represents a relationship between two tasks
type Dependency struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ParentID  string         `gorm:"size:20;not null;index:idx_parent" json:"parent_id"` // The blocking task
	ChildID   string         `gorm:"size:20;not null;index:idx_child;index:idx_child_type_parent,priority:1" json:"child_id"`  // The blocked task
	Type      string         `gorm:"size:20;default:blocks;index:idx_child_type_parent,priority:2" json:"type"`      // blocks, related, parent-child
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations (not stored, populated by queries)
	Parent *Task `gorm:"foreignKey:ParentID;references:ID" json:"parent,omitempty"`
	Child  *Task `gorm:"foreignKey:ChildID;references:ID" json:"child,omitempty"`
}

// TableName specifies the table name for Dependency
func (Dependency) TableName() string {
	return "dependencies"
}

// BeforeCreate validates the dependency before creation
func (d *Dependency) BeforeCreate(tx *gorm.DB) error {
	if d.Type == "" {
		d.Type = DepTypeBlocks
	}
	return nil
}

// IsBlocking returns true if this is a blocking dependency
func (d *Dependency) IsBlocking() bool {
	return d.Type == DepTypeBlocks
}
