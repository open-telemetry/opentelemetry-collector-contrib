// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: NewFactory().CreateDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				Endpoint: "apm.collector.apj-01.cloud.solarwinds.com:443",
				Key:      "something:name",
				Interval: time.Duration(10) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				Endpoint: "apm.collector.na-01.cloud.solarwinds.com:443",
				Key:      "something",
				Interval: time.Duration(5) * time.Second,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "3"),
			expected: &Config{
				Endpoint: "apm.collector.na-01.cloud.solarwinds.com:443",
				Key:      "something:name",
				Interval: time.Duration(60) * time.Second,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestResolveServiceNameBestEffort(t *testing.T) {
	// Without any environment variables
	require.Empty(t, resolveServiceNameBestEffort())
	// With OTEL_SERVICE_NAME only
	require.NoError(t, os.Setenv("OTEL_SERVICE_NAME", "otel_ser1"))
	require.Equal(t, "otel_ser1", resolveServiceNameBestEffort())
	require.NoError(t, os.Unsetenv("OTEL_SERVICE_NAME"))
	// With AWS_LAMBDA_FUNCTION_NAME only
	require.NoError(t, os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "lambda"))
	require.Equal(t, "lambda", resolveServiceNameBestEffort())
	require.NoError(t, os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME"))
	// With both
	require.NoError(t, os.Setenv("OTEL_SERVICE_NAME", "otel_ser1"))
	require.NoError(t, os.Setenv("AWS_LAMBDA_FUNCTION_NAME", "lambda"))
	require.Equal(t, "otel_ser1", resolveServiceNameBestEffort())
	require.NoError(t, os.Unsetenv("AWS_LAMBDA_FUNCTION_NAME"))
	require.NoError(t, os.Unsetenv("OTEL_SERVICE_NAME"))
}
