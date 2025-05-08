// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestNewMarshaller(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		shouldError bool
	}{
		{
			name: "valid json format",
			config: &Config{
				FormatType: formatTypeJSON,
				Encodings:  &Encodings{},
			},
			shouldError: false,
		},
		{
			name: "valid proto format",
			config: &Config{
				FormatType: formatTypeProto,
				Encodings:  &Encodings{},
			},
			shouldError: false,
		},
		{
			name: "invalid format",
			config: &Config{
				FormatType: "invalid",
				Encodings:  &Encodings{},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newMarshaller(tt.config, componenttest.NewNopHost())
			if tt.shouldError {
				assert.Error(t, err)
				assert.Nil(t, m)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, m)
			}
		})
	}
}

func TestMarshalTraces(t *testing.T) {
	tests := []struct {
		name        string
		formatType  string
		traces      ptrace.Traces
		shouldError bool
	}{
		{
			name:       "json format",
			formatType: formatTypeJSON,
			traces:     testdata.GenerateTracesTwoSpansSameResource(),
		},
		{
			name:       "proto format",
			formatType: formatTypeProto,
			traces:     testdata.GenerateTracesTwoSpansSameResource(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newMarshaller(&Config{FormatType: tt.formatType, Encodings: &Encodings{}}, componenttest.NewNopHost())
			require.NoError(t, err)

			data, err := m.marshalTraces(tt.traces)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)
		})
	}
}

func TestMarshalMetrics(t *testing.T) {
	tests := []struct {
		name        string
		formatType  string
		metrics     pmetric.Metrics
		shouldError bool
	}{
		{
			name:       "json format",
			formatType: formatTypeJSON,
			metrics:    testdata.GenerateMetricsTwoMetrics(),
		},
		{
			name:       "proto format",
			formatType: formatTypeProto,
			metrics:    testdata.GenerateMetricsTwoMetrics(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newMarshaller(&Config{FormatType: tt.formatType, Encodings: &Encodings{}}, componenttest.NewNopHost())
			require.NoError(t, err)

			data, err := m.marshalMetrics(tt.metrics)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)
		})
	}
}

func TestMarshalLogs(t *testing.T) {
	tests := []struct {
		name        string
		formatType  string
		logs        plog.Logs
		shouldError bool
	}{
		{
			name:       "json format",
			formatType: formatTypeJSON,
			logs:       testdata.GenerateLogsTwoLogRecordsSameResource(),
		},
		{
			name:       "proto format",
			formatType: formatTypeProto,
			logs:       testdata.GenerateLogsTwoLogRecordsSameResource(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := newMarshaller(&Config{FormatType: tt.formatType, Encodings: &Encodings{}}, componenttest.NewNopHost())
			require.NoError(t, err)

			data, err := m.marshalLogs(tt.logs)
			assert.NoError(t, err)
			assert.NotEmpty(t, data)
		})
	}
}
