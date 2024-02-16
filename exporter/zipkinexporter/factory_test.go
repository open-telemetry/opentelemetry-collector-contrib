// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateInstanceViaFactory(t *testing.T) {
	cfg := createDefaultConfig()

	// URL doesn't have a default value so set it directly.
	zeCfg := cfg.(*Config)
	zeCfg.Endpoint = "http://some.location.org:9411/api/v2/spans"
	ze, err := createTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, ze)
}
