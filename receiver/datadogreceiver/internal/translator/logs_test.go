// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestToPlog(t *testing.T) {
	tests := []struct {
		name         string
		incomingLogs []*DatadogLogPayload
		expectEmpty  bool
	}{
		{
			name:         "empty logs slice",
			incomingLogs: []*DatadogLogPayload{},
			expectEmpty:  true,
		},
		{
			name:         "nil logs slice",
			incomingLogs: nil,
			expectEmpty:  true,
		},
		{
			name: "single_log_entry",
			incomingLogs: []*DatadogLogPayload{
				{
					Message:   "Test log message",
					Status:    "info",
					Timestamp: 1234567890,
					Hostname:  "test-host",
					Service:   "test-service",
					Source:    "test-source",
					Tags:      "env:prod,version:1.0",
				},
			},
			expectEmpty: false,
		},
		{
			name: "multiple_log_entries",
			incomingLogs: []*DatadogLogPayload{
				{
					Message:   "First log",
					Status:    "info",
					Timestamp: 1000000000,
					Hostname:  "host1",
					Service:   "service1",
					Source:    "source1",
					Tags:      "tag1:value1",
				},
				{
					Message:   "Second log",
					Status:    "error",
					Timestamp: 2000000000,
					Hostname:  "host2",
					Service:   "service2",
					Source:    "source2",
					Tags:      "tag2:value2,tagB:valueB",
				},
				{
					Message:   "Third log",
					Status:    "warning",
					Timestamp: 3000000000,
					Hostname:  "host3",
					Service:   "service3",
					Source:    "source3",
					Tags:      "tag3:value3",
				},
			},
			expectEmpty: false,
		},
		{
			name: "log_with_empty_strings",
			incomingLogs: []*DatadogLogPayload{
				{
					Message:   "",
					Status:    "",
					Timestamp: 0,
					Hostname:  "",
					Service:   "",
					Source:    "",
					Tags:      "",
				},
			},
			expectEmpty: false,
		},
		{
			name: "log_with_special_characters",
			incomingLogs: []*DatadogLogPayload{
				{
					Message:   "Log with special chars: !@#$%^&*()_+-={}[]|\\:;\"'<>,.?/~`",
					Status:    "info",
					Timestamp: 1234567890,
					Hostname:  "host-with-dashes",
					Service:   "service_with_underscores",
					Source:    "source/with/slashes",
					Tags:      "key:value,key2:value2,special:!@#",
				},
			},
			expectEmpty: false,
		},
		{
			name: "log_with_negative_timestamp",
			incomingLogs: []*DatadogLogPayload{
				{
					Message:   "Log with negative timestamp",
					Status:    "info",
					Timestamp: -1234567890,
					Hostname:  "test-host",
					Service:   "test-service",
					Source:    "test-source",
					Tags:      "env:test",
				},
			},
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := ToPlog(tt.incomingLogs)

			// Validate empty result
			if tt.expectEmpty {
				assert.Equal(t, 0, actual.ResourceLogs().Len())
				return
			}

			expected, err := golden.ReadLogs(filepath.Join("testdata", "logs", tt.name+".yaml"))
			assert.NoError(t, err)
			assert.NoError(t, plogtest.CompareLogs(expected, actual, plogtest.IgnoreTimestamp()))
		})
	}
}
