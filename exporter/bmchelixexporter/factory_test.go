// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter

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

func TestCreateMetricsExporter(t *testing.T) {
	cfg := createDefaultConfig()
	_, err := createMetricsExporter(context.Background(), exportertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	exp, err := factory.CreateMetrics(
		context.Background(),
		exportertest.NewNopSettings(),
		cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}
