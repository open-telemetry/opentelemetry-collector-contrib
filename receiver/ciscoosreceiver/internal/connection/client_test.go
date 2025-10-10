// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestRPCClient_GetOSType(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name     string
		osType   string
		expected string
	}{
		{
			name:     "Returns configured OS type",
			osType:   "NX-OS",
			expected: "NX-OS",
		},
		{
			name:     "Returns default when empty",
			osType:   "",
			expected: "IOS XE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &RPCClient{
				OSType: tt.osType,
				Logger: logger,
			}
			assert.Equal(t, tt.expected, client.GetOSType())
		})
	}
}

func TestRPCClient_GetCommand(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name     string
		osType   string
		feature  string
		expected string
	}{
		{
			name:     "Interfaces command for NX-OS",
			osType:   "NX-OS",
			feature:  "interfaces",
			expected: "show interface",
		},
		{
			name:     "Interfaces command for IOS XE",
			osType:   "IOS XE",
			feature:  "interfaces",
			expected: "show interfaces",
		},
		{
			name:     "BGP command for NX-OS",
			osType:   "NX-OS",
			feature:  "bgp",
			expected: "show bgp all summary",
		},
		{
			name:     "BGP command for IOS",
			osType:   "IOS",
			feature:  "bgp",
			expected: "show ip bgp summary",
		},
		{
			name:     "Environment command",
			osType:   "IOS XE",
			feature:  "environment",
			expected: "show environment",
		},
		{
			name:     "Facts version command",
			osType:   "IOS XE",
			feature:  "facts_version",
			expected: "show version",
		},
		{
			name:     "Facts memory command for NX-OS",
			osType:   "NX-OS",
			feature:  "facts_memory",
			expected: "show system resources",
		},
		{
			name:     "Facts memory command for IOS XE",
			osType:   "IOS XE",
			feature:  "facts_memory",
			expected: "show memory statistics",
		},
		{
			name:     "VLANs command for IOS XE",
			osType:   "IOS XE",
			feature:  "interfaces_vlans",
			expected: "show vlans",
		},
		{
			name:     "VLANs command for NX-OS returns empty",
			osType:   "NX-OS",
			feature:  "interfaces_vlans",
			expected: "",
		},
		{
			name:     "Unknown feature returns empty",
			osType:   "IOS XE",
			feature:  "unknown",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &RPCClient{
				OSType: tt.osType,
				Logger: logger,
			}
			assert.Equal(t, tt.expected, client.GetCommand(tt.feature))
		})
	}
}
