// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateTracesExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateTracesExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateMetricsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateLogsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateMetricsExporterWithConnectionName(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.Name = "my-conn-name"

	te, err := factory.CreateMetricsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateExporterWithCustomRoutingKey(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Routing.RoutingKey = "custom_routing_key"

	te, err := factory.CreateLogsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateExporterWithConnectionName(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.Name = "my-conn-name"

	te, err := factory.CreateLogsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateExporterWithTLSSettings(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.TLSConfig = &configtls.ClientConfig{}

	te, err := factory.CreateLogsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateTracesExporterWithConnectionName(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.Name = "my-conn-name"

	te, err := factory.CreateTracesExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}
