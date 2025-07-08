// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDatabaseFromDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid DSN with database",
			dsn:      "clickhouse://localhost:9000/otel",
			expected: "otel",
			wantErr:  false,
		},
		{
			name:     "valid DSN without database",
			dsn:      "clickhouse://localhost:9000",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "valid DSN with default database",
			dsn:      "clickhouse://localhost:9000/default",
			expected: "default",
			wantErr:  false,
		},
		{
			name:     "invalid DSN format",
			dsn:      "invalid-dsn",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty DSN",
			dsn:      "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "DSN with special characters in database name",
			dsn:      "clickhouse://localhost:9000/test_otel-123",
			expected: "test_otel-123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DatabaseFromDSN(tt.dsn)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), "failed to parse DSN")
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestGenerateTTLExpr(t *testing.T) {
	tests := []struct {
		name      string
		ttl       time.Duration
		timeField string
		expected  string
	}{
		{
			name:      "zero TTL",
			ttl:       0,
			timeField: "timestamp",
			expected:  "",
		},
		{
			name:      "negative TTL",
			ttl:       -1 * time.Hour,
			timeField: "timestamp",
			expected:  "",
		},
		{
			name:      "1 day TTL",
			ttl:       24 * time.Hour,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalDay(1)",
		},
		{
			name:      "1 week TTL",
			ttl:       7 * 24 * time.Hour,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalDay(7)",
		},
		{
			name:      "1 hour TTL",
			ttl:       time.Hour,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalHour(1)",
		},
		{
			name:      "6 hours TTL",
			ttl:       6 * time.Hour,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalHour(6)",
		},
		{
			name:      "1 minute TTL",
			ttl:       time.Minute,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalMinute(1)",
		},
		{
			name:      "30 minute TTL",
			ttl:       30 * time.Minute,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalMinute(30)",
		},
		{
			name:      "1 second TTL",
			ttl:       time.Second,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalSecond(1)",
		},
		{
			name:      "45 second TTL",
			ttl:       45 * time.Second,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalSecond(45)",
		},
		{
			name:      "1500ms TTL",
			ttl:       1500 * time.Millisecond,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalSecond(1)",
		},
		{
			name:      "90 minute TTL",
			ttl:       90 * time.Minute,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalMinute(90)",
		},
		{
			name:      "25 hour TTL",
			ttl:       25 * time.Hour,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalHour(25)",
		},
		{
			name:      "time field with underscores",
			ttl:       time.Hour,
			timeField: "some_timestamp_column",
			expected:  "TTL some_timestamp_column + toIntervalHour(1)",
		},
		{
			name:      "1 year TTL",
			ttl:       365 * 24 * time.Hour,
			timeField: "timestamp",
			expected:  "TTL timestamp + toIntervalDay(365)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateTTLExpr(tt.ttl, tt.timeField)
			require.Equal(t, tt.expected, result)
		})
	}
}
