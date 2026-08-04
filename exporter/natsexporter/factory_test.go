// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/natstest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.NotNil(t, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	assert.Equal(t, defaultTracesSubject, cfg.Traces.Subject)
	assert.Equal(t, defaultMetricsSubject, cfg.Metrics.Subject)
	assert.Equal(t, defaultLogsSubject, cfg.Logs.Subject)
	assert.Equal(t, defaultEncoding, cfg.Traces.Encoding)
	assert.Equal(t, defaultConnectionTimeout, cfg.Connection.Timeout)
	assert.Equal(t, defaultMaxReconnects, cfg.Connection.MaxReconnects)
	assert.True(t, cfg.QueueSettings.HasValue())
	assert.False(t, cfg.JetStream.HasValue())
}

func TestFactory_CreateExporters(t *testing.T) {
	url := natstest.NewServer(t)

	tests := []struct {
		name   string
		create func(f exporter.Factory, cfg *Config) (component.Component, error)
	}{
		{
			name: "traces",
			create: func(f exporter.Factory, cfg *Config) (component.Component, error) {
				return f.CreateTraces(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			},
		},
		{
			name: "metrics",
			create: func(f exporter.Factory, cfg *Config) (component.Component, error) {
				return f.CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			},
		},
		{
			name: "logs",
			create: func(f exporter.Factory, cfg *Config) (component.Component, error) {
				return f.CreateLogs(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFactory()
			cfg := f.CreateDefaultConfig().(*Config)
			cfg.URLs = []string{url}

			exp, err := tt.create(f, cfg)
			require.NoError(t, err)
			require.NotNil(t, exp)

			require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
			assert.NoError(t, exp.Shutdown(t.Context()))
		})
	}
}
