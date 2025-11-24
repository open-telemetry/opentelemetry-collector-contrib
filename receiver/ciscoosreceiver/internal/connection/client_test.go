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
			name:     "Version command",
			osType:   "IOS XE",
			feature:  "version",
			expected: "show version",
		},
		{
			name:     "CPU command for NX-OS",
			osType:   "NX-OS",
			feature:  "cpu",
			expected: "show system resources",
		},
		{
			name:     "CPU command for IOS XE",
			osType:   "IOS XE",
			feature:  "cpu",
			expected: "show process cpu",
		},
		{
			name:     "Memory command for NX-OS",
			osType:   "NX-OS",
			feature:  "memory",
			expected: "show system resources",
		},
		{
			name:     "Memory command for IOS XE",
			osType:   "IOS XE",
			feature:  "memory",
			expected: "show process memory",
		},
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
			expected: "show interface",
		},
		{
			name:     "VLANs command for IOS XE returns empty (feature removed)",
			osType:   "IOS XE",
			feature:  "vlans",
			expected: "",
		},
		{
			name:     "VLANs command for NX-OS returns empty",
			osType:   "NX-OS",
			feature:  "vlans",
			expected: "",
		},
		{
			name:     "VLANs command for IOS returns empty",
			osType:   "IOS",
			feature:  "vlans",
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

func TestRPCClient_GetInterfaceCommands(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name               string
		osType             string
		expectedInterfaces string
		expectedVLANs      string
	}{
		{
			name:               "NX-OS interface commands",
			osType:             "NX-OS",
			expectedInterfaces: "show interface",
			expectedVLANs:      "",
		},
		{
			name:               "IOS XE interface commands (vlans feature removed)",
			osType:             "IOS XE",
			expectedInterfaces: "show interface",
			expectedVLANs:      "",
		},
		{
			name:               "IOS interface commands",
			osType:             "IOS",
			expectedInterfaces: "show interface",
			expectedVLANs:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &RPCClient{
				OSType: tt.osType,
				Logger: logger,
			}
			assert.Equal(t, tt.expectedInterfaces, client.GetCommand("interfaces"))
			assert.Equal(t, tt.expectedVLANs, client.GetCommand("vlans"))
		})
	}
}
