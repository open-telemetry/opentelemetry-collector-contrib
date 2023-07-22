// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))

	actual, ok := cfg.(*Config)
	require.True(t, ok, "invalid Config: %#v", cfg)
	assert.False(t, actual.hasMetricsEndpoint())
	assert.False(t, actual.hasTracesEndpoint())
	assert.False(t, actual.Metrics.ResourceAttrsIncluded)
	assert.False(t, actual.Metrics.AppTagsExcluded)
}

func TestCreateExporter(t *testing.T) {
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	params := exportertest.NewNopCreateSettings()
	cfg.Traces.Endpoint = "http://localhost:30001"
	te, err := createTracesExporter(context.Background(), params, cfg)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
}

func TestCreateMetricsExporter(t *testing.T) {
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	params := exportertest.NewNopCreateSettings()
	cfg.Metrics.Endpoint = "http://localhost:2878"
	te, err := createMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create metrics exporter")
}

func TestCreateTraceExporterNilConfigError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	_, err := createTracesExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateMetricsExporterNilConfigError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	_, err := createMetricsExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateTraceExporterInvalidEndpointError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Traces.Endpoint = "http:#$%^&#$%&#"
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateMetricsExporterInvalidEndpointError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Metrics.Endpoint = "http:#$%^&#$%&#"
	_, err := createMetricsExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateTraceExporterMissingPortError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Traces.Endpoint = "http://localhost"
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateMetricsExporterMissingPortError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Metrics.Endpoint = "http://localhost"
	_, err := createMetricsExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateTraceExporterInvalidPortError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Traces.Endpoint = "http://localhost:c42a"
	_, err := createTracesExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}

func TestCreateMetricsExporterInvalidPortError(t *testing.T) {
	params := exportertest.NewNopCreateSettings()
	defaultConfig := createDefaultConfig()
	cfg := defaultConfig.(*Config)
	cfg.Metrics.Endpoint = "http://localhost:c42a"
	_, err := createMetricsExporter(context.Background(), params, cfg)
	assert.Error(t, err)
}
