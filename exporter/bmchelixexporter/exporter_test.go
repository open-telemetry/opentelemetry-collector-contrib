// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestNewBmcHelixExporterWithNilConfig(t *testing.T) {
	t.Parallel()

	exp, err := newBmcHelixExporter(nil, exportertest.NewNopSettings())
	assert.Nil(t, exp)
	assert.Error(t, err)
}

func TestNewBmcHelixExporterWithDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := createDefaultConfig().(*Config)
	exp, err := newBmcHelixExporter(cfg, exportertest.NewNopSettings())
	assert.NotNil(t, exp)
	assert.NoError(t, err)
}
