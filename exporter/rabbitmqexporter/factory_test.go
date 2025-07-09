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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateTraces(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateMetrics(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	te, err := factory.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateMetricsWithConnectionName(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.Name = "my-conn-name"

	te, err := factory.CreateMetrics(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateExporterWithCustomRoutingKey(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Routing.RoutingKey = "custom_routing_key"

	te, err := factory.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateExporterWithConnectionName(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.Name = "my-conn-name"

	te, err := factory.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateExporterWithTLSs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.TLSConfig = &configtls.ClientConfig{}

	te, err := factory.CreateLogs(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateTracesWithConnectionName(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Connection.Name = "my-conn-name"

	te, err := factory.CreateTraces(context.Background(), exportertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}
