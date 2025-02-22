// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/metadata"
)

func TestNewMetricsExporterWithNilConfig(t *testing.T) {
	t.Parallel()

	exp, err := newMetricsExporter(nil, exportertest.NewNopSettings(metadata.Type))
	assert.Nil(t, exp)
	assert.Error(t, err)
}

func TestNewMetricsExporterWithDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := createDefaultConfig().(*Config)
	exp, err := newMetricsExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, exp)
	assert.NoError(t, err)
}
