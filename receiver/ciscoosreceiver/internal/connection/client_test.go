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
