// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	require.NotNil(t, cfg)
	assert.NoError(t, confmap.Validate(cfg))
}

func TestCreateExporters(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := exportertest.NewNopSettings(factory.Type())

	le, err := factory.CreateLogs(t.Context(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, le)

	me, err := factory.CreateMetrics(t.Context(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, me)

	te, err := factory.CreateTraces(t.Context(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, te)
}

// The logs, metrics, and traces exporters built from one config must share a
// single natsExporter (and so a single NATS connection) via sharedcomponent.
func TestCreateExportersShareInstance(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := exportertest.NewNopSettings(factory.Type())

	first := getOrCreateExporter(set, cfg).Unwrap()
	for range 2 {
		assert.Same(t, first, getOrCreateExporter(set, cfg).Unwrap())
	}

	// A different config gets its own instance.
	other := getOrCreateExporter(set, factory.CreateDefaultConfig()).Unwrap()
	assert.NotSame(t, first, other)
}
