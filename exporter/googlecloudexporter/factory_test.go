// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudexporter

import (
	"context"
	"os"
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
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Default credentials not set, skip creating Google Cloud exporter")
	}
	ctx := context.Background()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.ProjectID = "test"

	te, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}

func TestCreateLegacyExporter(t *testing.T) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		t.Skip("Default credentials not set, skip creating Google Cloud exporter")
	}
	ctx := context.Background()
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.ProjectID = "test"

	te, err := factory.CreateTraces(ctx, exportertest.NewNopSettings(), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := factory.CreateMetrics(ctx, exportertest.NewNopSettings(), eCfg)
	assert.NoError(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}
