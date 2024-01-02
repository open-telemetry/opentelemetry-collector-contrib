// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sapmexporter

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
	assert.Equal(t, "sapm", string(factory.Type()))

	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.Endpoint = "http://local"
	params := exportertest.NewNopCreateSettings()

	te, err := factory.CreateTracesExporter(context.Background(), params, eCfg)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := factory.CreateMetricsExporter(context.Background(), params, eCfg)
	assert.Error(t, err)
	assert.Nil(t, me)
}
