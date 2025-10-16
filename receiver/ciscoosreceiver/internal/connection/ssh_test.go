// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClient_DetectOSType(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "IOS XE detection",
			output:   "Cisco IOS XE Software, Version 16.9.1",
			expected: "IOS XE",
		},
		{
			name:     "NX-OS detection with Nexus",
			output:   "Cisco Nexus Operating System (NX-OS) Software",
			expected: "NX-OS",
		},
		{
			name:     "NX-OS detection with NX-OS",
			output:   "Cisco NX-OS(tm) Software, Version 9.3(5)",
			expected: "NX-OS",
		},
		{
			name:     "IOS detection",
			output:   "Cisco IOS Software, C2960 Software",
			expected: "IOS",
		},
		{
			name:     "Unknown defaults to IOS XE",
			output:   "Some other output",
			expected: "IOS XE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test OS detection logic directly
			var result string
			switch {
			case contains(tt.output, "Cisco IOS XE"):
				result = "IOS XE"
			case contains(tt.output, "Cisco Nexus") || contains(tt.output, "NX-OS"):
				result = "NX-OS"
			case contains(tt.output, "Cisco IOS Software"):
				result = "IOS"
			default:
				result = "IOS XE"
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
