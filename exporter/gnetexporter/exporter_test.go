// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)
	require.Equal(t, "gnet", f.Type().String())

	cfg := f.CreateDefaultConfig()
	c, ok := cfg.(*Config)
	require.True(t, ok)
	require.Equal(t, "localhost:4320", c.Endpoint)
}

func TestCreateExporters(t *testing.T) {
	f := NewFactory()
	params := exportertest.NewNopSettings(f.Type())

	te, err := f.CreateTraces(context.Background(), params, f.CreateDefaultConfig())
	require.NoError(t, err)
	require.NotNil(t, te)

	me, err := f.CreateMetrics(context.Background(), params, f.CreateDefaultConfig())
	require.NoError(t, err)
	require.NotNil(t, me)

	le, err := f.CreateLogs(context.Background(), params, f.CreateDefaultConfig())
	require.NoError(t, err)
	require.NotNil(t, le)
}

func TestConfigValidate(t *testing.T) {
	c := &Config{
		Endpoint:     "localhost:4320",
		DialTimeout:  defaultDialTimeout,
		WriteTimeout: defaultWriteTimeout,
	}
	require.NoError(t, c.Validate())

	c.Endpoint = ""
	require.Error(t, c.Validate())

	c.Endpoint = "localhost:4320"
	c.DialTimeout = 0
	require.Error(t, c.Validate())
}
