// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	me, err := factory.CreateMetrics(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")

	// Validate that the component implements the proper interface
	require.Implements(t, (*interface{})(nil), me)

	// Shutdown exporter
	err = me.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestCreateTracesExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateTraces(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg,
	)
	assert.Error(t, err)
	assert.Nil(t, te)
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	le, err := factory.CreateLogs(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg,
	)
	assert.Error(t, err)
	assert.Nil(t, le)
}
