// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	exp, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig()
	exp, err := createTracesExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}

func TestCreateLogsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	exp, err := createLogsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)
}
