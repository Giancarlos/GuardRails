package models

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"time"

	"gorm.io/gorm"
)

// Gate result constants
const (
	GatePending = "pending"
	GatePassed  = "passed"
	GateFailed  = "failed"
	GateSkipped = "skipped"
)

// Common gate types (not enforced, just suggestions)
// Users can use any type string they want: test, review, approval, manual, deploy, qa, doc, etc.

// Gate ID constants
const (
	GateIDByteLength = 4
	GateIDPrefix     = "gate-"
)

// Pass rate calculation constant
const GatePercentMultiplier = 100

// Gate ID validation pattern
var gateIDPattern = regexp.MustCompile(`^gate-[a-f0-9]{8}$`)

// ValidateGateID validates that a gate ID has the correct format
func ValidateGateID(id string) bool {
	return gateIDPattern.MatchString(id)
}

// Gate represents a quality gate that must pass before task completion
type Gate struct {
	ID             string         `gorm:"primaryKey;size:20" json:"id"`
	Title          string         `gorm:"size:255;not null" json:"title"`
	Description    string         `gorm:"type:text" json:"description,omitempty"`
	Category       string         `gorm:"size:100;index" json:"category,omitempty"`   // e.g., "auth", "api", "ui"
	Type           string         `gorm:"size:20;default:manual" json:"type"`         // test, review, approval, manual, deploy, qa, doc
	Priority       int            `gorm:"index" json:"priority"`                      // 0=critical, 4=lowest
	Preconditions  string         `gorm:"type:text" json:"preconditions,omitempty"`   // Setup required
	Steps          string         `gorm:"type:text" json:"steps,omitempty"`           // Instructions
	ExpectedResult string         `gorm:"type:text" json:"expected_result,omitempty"` // What should happen
	Command        string         `gorm:"type:text" json:"command,omitempty"`         // Command to run for automated gates
	Labels         StringSlice    `gorm:"type:text" json:"labels,omitempty"`
	LastResult     string         `gorm:"size:20;default:pending" json:"last_result"` // pending, passed, failed, skipped
	LastRunAt      *time.Time     `json:"last_run_at,omitempty"`
	LastRunBy      string         `gorm:"size:100" json:"last_run_by,omitempty"`     // "human" or "agent" or specific name
	LastRunNotes   string         `gorm:"type:text" json:"last_run_notes,omitempty"` // Notes from last run
	RunCount       int            `gorm:"default:0" json:"run_count"`
	PassCount      int            `gorm:"default:0" json:"pass_count"`
	FailCount      int            `gorm:"default:0" json:"fail_count"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Gate
func (Gate) TableName() string {
	return "gates"
}

// Gate link status constants
const (
	GateLinkPending = "pending"
	GateLinkPassed  = "passed"
	GateLinkFailed  = "failed"
)

// GateTaskLink links gates to tasks (many-to-many)
// Each link has its own verification status - gates must be verified per-task
type GateTaskLink struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	GateID     string         `gorm:"size:20;not null;index" json:"gate_id"`
	TaskID     string         `gorm:"size:20;not null;index" json:"task_id"`
	Status     string         `gorm:"size:20;default:pending" json:"status"` // pending, passed, failed
	VerifiedAt *time.Time     `json:"verified_at,omitempty"`
	VerifiedBy string         `gorm:"size:100" json:"verified_by,omitempty"` // human, agent, or name
	Notes      string         `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt  time.Time      `gorm:"autoCreateTime" json:"created_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for GateTaskLink
func (GateTaskLink) TableName() string {
	return "gate_task_links"
}

// GateRun records each execution of a gate
type GateRun struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GateID    string    `gorm:"size:20;not null;index" json:"gate_id"`
	Result    string    `gorm:"size:20;not null" json:"result"` // passed, failed, skipped
	RunBy     string    `gorm:"size:100" json:"run_by"`         // "human", "agent", or name
	Notes     string    `gorm:"type:text" json:"notes,omitempty"`
	Duration  int       `json:"duration_ms,omitempty"`             // Duration in milliseconds
	Output    string    `gorm:"type:text" json:"output,omitempty"` // Command output for automated gates
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName specifies the table name for GateRun
func (GateRun) TableName() string {
	return "gate_runs"
}

// GenerateGateID creates a new hash-based gate ID like "gate-a1b2c3d4"
func GenerateGateID() string {
	bytes := make([]byte, GateIDByteLength)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand failure indicates serious system issues - fail fast
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return GateIDPrefix + hex.EncodeToString(bytes)
}

// BeforeCreate hook to generate ID if not set
func (g *Gate) BeforeCreate(tx *gorm.DB) error {
	if g.ID == "" {
		g.ID = GenerateGateID()
	}
	return nil
}

// RecordRun updates the gate with run results
func (g *Gate) RecordRun(result, runBy, notes string) {
	now := time.Now()
	g.LastResult = result
	g.LastRunAt = &now
	g.LastRunBy = runBy
	g.LastRunNotes = notes
	g.RunCount++
	if result == GatePassed {
		g.PassCount++
	} else if result == GateFailed {
		g.FailCount++
	}
}

// PassRate returns the pass rate as a percentage
func (g *Gate) PassRate() float64 {
	if g.RunCount == 0 {
		return 0
	}
	return float64(g.PassCount) / float64(g.RunCount) * GatePercentMultiplier
}

// TypeString returns the type (as-is, free-form)
func (g *Gate) TypeString() string {
	if g.Type == "" {
		return "manual"
	}
	return g.Type
}

// ResultString returns a human-readable result
func (g *Gate) ResultString() string {
	switch g.LastResult {
	case GatePassed:
		return "PASS"
	case GateFailed:
		return "FAIL"
	case GateSkipped:
		return "SKIP"
	default:
		return "PENDING"
	}
}
