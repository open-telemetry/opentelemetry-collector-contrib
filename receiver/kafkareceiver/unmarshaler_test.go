// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDefaultTracesUnMarshaler(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
		"jaeger_proto",
		"jaeger_json",
		"zipkin_proto",
		"zipkin_json",
		"zipkin_thrift",
	}
	marshalers := defaultTracesUnmarshalers()
	assert.Len(t, marshalers, len(expectedEncodings))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestDefaultMetricsUnMarshaler(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
	}
	marshalers := defaultMetricsUnmarshalers()
	assert.Len(t, marshalers, len(expectedEncodings))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}

func TestDefaultLogsUnMarshaler(t *testing.T) {
	expectedEncodings := []string{
		"otlp_proto",
		"otlp_json",
		"raw",
		"text",
		"json",
		"azure_resource_logs",
	}
	marshalers := defaultLogsUnmarshalers("Test Version", zap.NewNop())
	assert.Len(t, marshalers, len(expectedEncodings))
	for _, e := range expectedEncodings {
		t.Run(e, func(t *testing.T) {
			m, ok := marshalers[e]
			require.True(t, ok)
			assert.NotNil(t, m)
		})
	}
}
