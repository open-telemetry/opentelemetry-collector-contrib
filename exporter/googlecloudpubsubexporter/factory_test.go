// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}

func TestCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.Endpoint = "http://testing.invalid"

	te, err := factory.CreateTraces(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		eCfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.Endpoint = "http://testing.invalid"

	me, err := factory.CreateMetrics(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		eCfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}

func TestLogsCreateExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.Endpoint = "http://testing.invalid"

	me, err := factory.CreateLogs(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		eCfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create logs exporter")
}

func TestEnsureExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.Endpoint = "http://testing.invalid"

	exporter1 := ensureExporter(exportertest.NewNopSettings(metadata.Type), eCfg)
	exporter2 := ensureExporter(exportertest.NewNopSettings(metadata.Type), eCfg)
	assert.Equal(t, exporter1, exporter2)
}
