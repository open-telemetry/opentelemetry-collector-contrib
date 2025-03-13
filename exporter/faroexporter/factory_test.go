// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "https://faro.example.com/collect"

	set := exportertest.NewNopSettings()
	te, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	le, err := factory.CreateLogsExporter(context.Background(), set, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, le, "failed to create logs exporter")

	me, err := factory.CreateMetricsExporter(context.Background(), set, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}
