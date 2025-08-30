// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Connection.Endpoint = "tcp://localhost:1883"
	cfg.Connection.Auth.Plain.Username = "test"
	cfg.Connection.Auth.Plain.Password = "test"

	set := exportertest.NewNopSettings(metadata.Type)
	te, err := createTracesExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Connection.Endpoint = "tcp://localhost:1883"
	cfg.Connection.Auth.Plain.Username = "test"
	cfg.Connection.Auth.Plain.Password = "test"

	set := exportertest.NewNopSettings(metadata.Type)
	me, err := createMetricsExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Connection.Endpoint = "tcp://localhost:1883"
	cfg.Connection.Auth.Plain.Username = "test"
	cfg.Connection.Auth.Plain.Password = "test"

	set := exportertest.NewNopSettings(metadata.Type)
	le, err := createLogsExporter(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, le, "failed to create logs exporter")
}

func TestCreateTracesExporterWithInvalidConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// Set invalid endpoint that will cause connection to fail
	cfg.Connection.Endpoint = "invalid://endpoint"
	set := exportertest.NewNopSettings(metadata.Type)
	te, err := createTracesExporter(context.Background(), set, cfg)
	// The exporter should still be created, but will fail during start
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateMetricsExporterWithInvalidConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// Set invalid endpoint that will cause connection to fail
	cfg.Connection.Endpoint = "invalid://endpoint"
	set := exportertest.NewNopSettings(metadata.Type)
	me, err := createMetricsExporter(context.Background(), set, cfg)
	// The exporter should still be created, but will fail during start
	assert.NoError(t, err)
	assert.NotNil(t, me)
}

func TestCreateLogsExporterWithInvalidConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	// Set invalid endpoint that will cause connection to fail
	cfg.Connection.Endpoint = "invalid://endpoint"
	set := exportertest.NewNopSettings(metadata.Type)
	le, err := createLogsExporter(context.Background(), set, cfg)
	// The exporter should still be created, but will fail during start
	assert.NoError(t, err)
	assert.NotNil(t, le)
}

func TestGetTopicOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		fallback string
		expected string
	}{
		{
			name: "use config topic",
			config: &Config{
				Topic: TopicConfig{
					Topic: "custom/topic",
				},
			},
			fallback: "default/topic",
			expected: "custom/topic",
		},
		{
			name: "use fallback topic",
			config: &Config{
				Topic: TopicConfig{
					Topic: "",
				},
			},
			fallback: "default/topic",
			expected: "default/topic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getTopicOrDefault(tt.config, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}
