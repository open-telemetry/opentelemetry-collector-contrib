// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg.(*Config).logger)
}

func TestCreateTraces(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "1").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	ctx := context.Background()
	exporter, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(), cfg)
	assert.Error(t, err)
	assert.Nil(t, exporter)
}

func TestCreateMetrics(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "1").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	ctx := context.Background()
	exporter, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exporter)
}
