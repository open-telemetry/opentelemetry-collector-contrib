// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig()
	exp, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig()
	exp, err := createTracesExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig()
	exp, err := createLogsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestUnsupportedMarshalerOptions(t *testing.T) {
	cfg := createDefaultConfig()
	cfg.(*Config).MarshalerName = SumoIC
	exp, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.Error(t, err)
	require.Nil(t, exp)

	exp2, err := createTracesExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.Error(t, err)
	require.Nil(t, exp2)
}
