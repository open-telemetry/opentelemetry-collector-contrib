// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerthrifthttpexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateInstanceViaFactory(t *testing.T) {
	cfg := createDefaultConfig()
	params := exportertest.NewNopCreateSettings()
	// Endpoint doesn't have a default value so set it directly.
	expCfg := cfg.(*Config)
	expCfg.HTTPClientSettings.Endpoint = "http://jaeger.example.com:12345/api/traces"
	exp, err := createTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestFactory_CreateTracesExporter(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://jaeger.example.com/api/traces",
			Headers: map[string]configopaque.String{
				"added-entry": "added value",
				"dot.test":    "test",
			},
			Timeout: 2 * time.Second,
		},
	}

	params := exportertest.NewNopCreateSettings()
	te, err := createTracesExporter(context.Background(), params, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}
