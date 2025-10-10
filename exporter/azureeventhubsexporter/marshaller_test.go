// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestJSONMarshaller(t *testing.T) {
	marshaller := newJSONMarshaller()
	assert.NotNil(t, marshaller)
	assert.Equal(t, formatTypeJSON, marshaller.format())

	t.Run("marshal traces", func(t *testing.T) {
		traces := testdata.GenerateTracesTwoSpansSameResource()
		data, err := marshaller.MarshalTraces(traces)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		// Verify it's valid JSON by checking first character
		assert.Equal(t, byte('{'), data[0])
	})

	t.Run("marshal logs", func(t *testing.T) {
		logs := testdata.GenerateLogsTwoLogRecordsSameResource()
		data, err := marshaller.MarshalLogs(logs)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		// Verify it's valid JSON
		assert.Equal(t, byte('{'), data[0])
	})

	t.Run("marshal metrics", func(t *testing.T) {
		metrics := testdata.GenerateMetricsTwoMetrics()
		data, err := marshaller.MarshalMetrics(metrics)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		// Verify it's valid JSON
		assert.Equal(t, byte('{'), data[0])
	})
}

func TestProtoMarshaller(t *testing.T) {
	marshaller := newProtoMarshaller()
	assert.NotNil(t, marshaller)
	assert.Equal(t, formatTypeProto, marshaller.format())

	t.Run("marshal traces", func(t *testing.T) {
		traces := testdata.GenerateTracesTwoSpansSameResource()
		data, err := marshaller.MarshalTraces(traces)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("marshal logs", func(t *testing.T) {
		logs := testdata.GenerateLogsTwoLogRecordsSameResource()
		data, err := marshaller.MarshalLogs(logs)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})

	t.Run("marshal metrics", func(t *testing.T) {
		metrics := testdata.GenerateMetricsTwoMetrics()
		data, err := marshaller.MarshalMetrics(metrics)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
	})
}

func TestCreateMarshaller(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		expectType  string
	}{
		{
			name: "json marshaller",
			config: &Config{
				FormatType: formatTypeJSON,
			},
			expectError: false,
			expectType:  formatTypeJSON,
		},
		{
			name: "proto marshaller",
			config: &Config{
				FormatType: formatTypeProto,
			},
			expectError: false,
			expectType:  formatTypeProto,
		},
		{
			name: "unsupported format",
			config: &Config{
				FormatType: "xml",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshaller, err := createMarshaller(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, marshaller)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, marshaller)
				assert.Equal(t, tt.expectType, marshaller.format())
			}
		})
	}
}
