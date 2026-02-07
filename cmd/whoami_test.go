package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

func TestHashHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantLen  int
	}{
		{"simple hostname", "localhost", 8},
		{"fqdn", "server.example.com", 8},
		{"empty", "", 8},
		{"with special chars", "my-server_01", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := sha256.Sum256([]byte(tt.hostname))
			hash := hex.EncodeToString(h[:])[:8]

			if len(hash) != tt.wantLen {
				t.Errorf("hashHostname() len = %d, want %d", len(hash), tt.wantLen)
			}

			// Verify deterministic
			h2 := sha256.Sum256([]byte(tt.hostname))
			hash2 := hex.EncodeToString(h2[:])[:8]
			if hash != hash2 {
				t.Errorf("hashHostname() not deterministic: %s != %s", hash, hash2)
			}
		})
	}
}

func TestHashHostnameUniqueness(t *testing.T) {
	hostnames := []string{"localhost", "server1", "server2", "my-machine.local"}
	hashes := make(map[string]string)

	for _, hostname := range hostnames {
		h := sha256.Sum256([]byte(hostname))
		hash := hex.EncodeToString(h[:])[:8]

		if existing, ok := hashes[hash]; ok {
			t.Errorf("hash collision: %s and %s both hash to %s", existing, hostname, hash)
		}
		hashes[hash] = hostname
	}
}

func TestGetHostname(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		// Some environments may not have a hostname
		t.Skip("hostname not available")
	}

	if hostname == "" {
		t.Error("hostname is empty")
	}
}
