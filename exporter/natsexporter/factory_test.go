// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"context"
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

	le, err := factory.CreateLogs(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, le)

	me, err := factory.CreateMetrics(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, me)

	te, err := factory.CreateTraces(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, te)
}
