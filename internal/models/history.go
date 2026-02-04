package models

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// TaskHistory records changes to tasks
type TaskHistory struct {
	ID        string    `gorm:"primaryKey;size:30" json:"id"`
	TaskID    string    `gorm:"size:20;index;not null" json:"task_id"`
	Field     string    `gorm:"size:50;not null" json:"field"`
	OldValue  string    `gorm:"type:text" json:"old_value,omitempty"`
	NewValue  string    `gorm:"type:text" json:"new_value,omitempty"`
	ChangedBy string    `gorm:"size:100" json:"changed_by,omitempty"`
	ChangedAt time.Time `gorm:"autoCreateTime" json:"changed_at"`
}

// GenerateHistoryID creates a new history entry ID
func GenerateHistoryID() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand failure indicates serious system issues - fail fast
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return "hist-" + hex.EncodeToString(bytes)
}

// BeforeCreate hook to generate ID
func (h *TaskHistory) BeforeCreate(tx *gorm.DB) error {
	if h.ID == "" {
		h.ID = GenerateHistoryID()
	}
	return nil
}

// RecordChange creates a history entry for a field change
func RecordChange(db *gorm.DB, taskID, field, oldValue, newValue, changedBy string) error {
	if oldValue == newValue {
		return nil // No change
	}
	entry := &TaskHistory{
		TaskID:    taskID,
		Field:     field,
		OldValue:  oldValue,
		NewValue:  newValue,
		ChangedBy: changedBy,
	}
	return db.Create(entry).Error
}
