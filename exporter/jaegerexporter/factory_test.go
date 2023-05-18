// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateMetricsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := exportertest.NewNopCreateSettings()
	_, err := factory.CreateMetricsExporter(context.Background(), set, cfg)
	assert.Error(t, err, component.ErrDataTypeIsNotSupported)
}

func TestCreateInstanceViaFactory(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()

	// Default config doesn't have default endpoint so creating from it should
	// fail.
	set := exportertest.NewNopCreateSettings()
	// Endpoint doesn't have a default value so set it directly.
	expCfg := cfg.(*Config)
	expCfg.Endpoint = "some.target.org:12345"
	exp, err := factory.CreateTracesExporter(context.Background(), set, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}
