// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter/internal/metadata"
)

const defaultURL = `https://api.example.com/v1`

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactory_CreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := withDefaultConfig(func(cfg *Config) {
		cfg.APIURL = defaultURL
	})
	params := exportertest.NewNopSettings(metadata.Type)
	exporter, err := factory.CreateLogs(context.Background(), params, cfg)
	require.NoError(t, err)
	require.NotNil(t, exporter)

	require.NoError(t, exporter.Shutdown(context.TODO()))
}
