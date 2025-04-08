// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter/internal/metadata"
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
	cfg.Endpoint = "https://faro.example.com/collect"

	set := exporter.Settings{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		BuildInfo:         component.NewDefaultBuildInfo(),
		ID:                component.NewID(metadata.Type),
	}

	ctx := context.Background()
	te, err := factory.CreateTraces(ctx, set, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	le, err := factory.CreateLogs(ctx, set, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, le, "failed to create logs exporter")

	// Metrics are not supported according to metadata.yaml
	// Skip testing metrics exporter
}
