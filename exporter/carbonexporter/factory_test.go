// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig()
	_, err := createMetricsExporter(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	exp, err := factory.CreateMetrics(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	// Set values that don't have a valid default.
	// expCfg := cfg.(*Config)

	exp, err = factory.CreateMetrics(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		cfg)
	assert.NoError(t, err)
	require.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}
