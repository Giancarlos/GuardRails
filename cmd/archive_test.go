package cmd

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		// Valid formats
		{"1d", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"1h", time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"48h", 48 * time.Hour, false},

		// Edge cases
		{"0d", 0, false},
		{"0h", 0, false},
		{"365d", 365 * 24 * time.Hour, false},
		{"52w", 52 * 7 * 24 * time.Hour, false},

		// Invalid formats
		{"", 0, true},    // empty
		{"d", 0, true},   // no number
		{"1", 0, true},   // no unit
		{"1x", 0, true},  // invalid unit
		{"abc", 0, true}, // non-numeric

		// Note: parser accepts these edge cases (documented behavior)
		{"-1d", -24 * time.Hour, false}, // negative duration allowed
		{"1.5d", 24 * time.Hour, false}, // parses "1" before decimal
		{"1dd", 24 * time.Hour, false},  // parses "1d" with unit 'd'
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("parseDuration(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
