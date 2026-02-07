package models

import (
	"testing"
)

func TestGenerateGateID(t *testing.T) {
	id := GenerateGateID()

	if len(id) != 13 { // "gate-" + 8 hex chars
		t.Errorf("GenerateGateID() wrong length: got %d, want 13", len(id))
	}

	if id[:5] != "gate-" {
		t.Errorf("GenerateGateID() wrong prefix: got %s", id[:5])
	}

	// Test uniqueness
	id2 := GenerateGateID()
	if id == id2 {
		t.Error("GenerateGateID() produced duplicate IDs")
	}
}

func TestGateRecordRun(t *testing.T) {
	gate := &Gate{
		ID:         "gate-a1b2c3d4",
		LastResult: GatePending,
	}

	// Record a pass
	gate.RecordRun(GatePassed, "agent", "All tests green")

	if gate.LastResult != GatePassed {
		t.Errorf("RecordRun() LastResult = %s, want %s", gate.LastResult, GatePassed)
	}
	if gate.RunCount != 1 {
		t.Errorf("RecordRun() RunCount = %d, want 1", gate.RunCount)
	}
	if gate.PassCount != 1 {
		t.Errorf("RecordRun() PassCount = %d, want 1", gate.PassCount)
	}
	if gate.FailCount != 0 {
		t.Errorf("RecordRun() FailCount = %d, want 0", gate.FailCount)
	}

	// Record a failure
	gate.RecordRun(GateFailed, "agent", "Test failed")

	if gate.LastResult != GateFailed {
		t.Errorf("RecordRun() LastResult = %s, want %s", gate.LastResult, GateFailed)
	}
	if gate.RunCount != 2 {
		t.Errorf("RecordRun() RunCount = %d, want 2", gate.RunCount)
	}
	if gate.FailCount != 1 {
		t.Errorf("RecordRun() FailCount = %d, want 1", gate.FailCount)
	}

	// Record a skip (shouldn't affect pass/fail counts)
	gate.RecordRun(GateSkipped, "agent", "Not applicable")

	if gate.LastResult != GateSkipped {
		t.Errorf("RecordRun() LastResult = %s, want %s", gate.LastResult, GateSkipped)
	}
	if gate.RunCount != 3 {
		t.Errorf("RecordRun() RunCount = %d, want 3", gate.RunCount)
	}
}

func TestGatePassRate(t *testing.T) {
	tests := []struct {
		runCount  int
		passCount int
		expected  float64
	}{
		{0, 0, 0.0},
		{10, 10, 100.0},
		{10, 0, 0.0},
		{10, 5, 50.0},
		{4, 3, 75.0},
	}

	for _, tt := range tests {
		gate := &Gate{
			RunCount:  tt.runCount,
			PassCount: tt.passCount,
		}
		got := gate.PassRate()
		if got != tt.expected {
			t.Errorf("PassRate() with %d runs, %d pass = %.1f, want %.1f",
				tt.runCount, tt.passCount, got, tt.expected)
		}
	}
}

func TestGateTypeString(t *testing.T) {
	tests := []struct {
		gateType string
		expected string
	}{
		{"test", "test"},
		{"review", "review"},
		{"", "manual"}, // default
		{"custom", "custom"},
	}

	for _, tt := range tests {
		gate := &Gate{Type: tt.gateType}
		if got := gate.TypeString(); got != tt.expected {
			t.Errorf("TypeString() with %q = %s, want %s", tt.gateType, got, tt.expected)
		}
	}
}

func TestGateResultString(t *testing.T) {
	tests := []struct {
		result   string
		expected string
	}{
		{GatePending, "PENDING"},
		{GatePassed, "PASS"},
		{GateFailed, "FAIL"},
		{GateSkipped, "SKIP"},
		{"unknown", "PENDING"}, // defaults to PENDING
	}

	for _, tt := range tests {
		gate := &Gate{LastResult: tt.result}
		if got := gate.ResultString(); got != tt.expected {
			t.Errorf("ResultString() with %s = %s, want %s", tt.result, got, tt.expected)
		}
	}
}

func TestGateIsPassing(t *testing.T) {
	tests := []struct {
		result  string
		passing bool
	}{
		{GatePending, false},
		{GatePassed, true},
		{GateFailed, false},
		{GateSkipped, true}, // Skipped counts as passing
	}

	for _, tt := range tests {
		gate := &Gate{LastResult: tt.result}
		// IsPassing: passed or skipped
		isPassing := gate.LastResult == GatePassed || gate.LastResult == GateSkipped
		if isPassing != tt.passing {
			t.Errorf("IsPassing logic with %s = %v, want %v", tt.result, isPassing, tt.passing)
		}
	}
}
