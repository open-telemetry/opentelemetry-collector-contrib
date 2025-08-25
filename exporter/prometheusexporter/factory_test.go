// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/otel/attribute"
	ometric "go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Endpoint = ""
	exp, err := createMetricsExporter(
		t.Context(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	require.Equal(t, errBlankPrometheusAddress, err)
	require.Nil(t, exp)
}

func TestCreateMetricsExportHelperError(t *testing.T) {
	cfg, ok := createDefaultConfig().(*Config)
	require.True(t, ok)

	cfg.Endpoint = "http://localhost:8889"

	set := exportertest.NewNopSettings(metadata.Type)
	set.Logger = nil

	// Should give us an exporterhelper.errNilLogger
	exp, err := createMetricsExporter(t.Context(), set, cfg)

	assert.Nil(t, exp)
	assert.Error(t, err)
}

func TestTelemetryInitialization(t *testing.T) {
	settings := exportertest.NewNopSettings(metadata.Type)

	// Test successful telemetry creation
	tel := newTelemetry(settings)
	require.NotNil(t, tel)

	// Verify the counter was created
	assert.NotNil(t, tel.refusedMetricPoints)

	// Test that we can add to the counter without errors (using the OTel metric API)
	tel.refusedMetricPoints.Add(t.Context(), 1,
		ometric.WithAttributes(
			attribute.String("exporter", "prometheus"),
			attribute.String("reason", "test"),
		))
}

func TestCreateMetricsExporterWithTelemetry(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "localhost:8888"

	settings := exportertest.NewNopSettings(metadata.Type)
	exporter, err := createMetricsExporter(t.Context(), settings, cfg)

	require.NoError(t, err)
	assert.NotNil(t, exporter)
}
