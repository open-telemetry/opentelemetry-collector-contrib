// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

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
		context.Background(),
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
	exp, err := createMetricsExporter(context.Background(), set, cfg)

	assert.Nil(t, exp)
	assert.Error(t, err)
}
