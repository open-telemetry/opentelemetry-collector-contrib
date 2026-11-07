// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gnetreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestFactory(t *testing.T) {
	f := NewFactory()
	require.NotNil(t, f)
	require.Equal(t, "gnet", f.Type().String())

	cfg := f.CreateDefaultConfig()
	c, ok := cfg.(*Config)
	require.True(t, ok)
	require.Equal(t, defaultEndpoint, c.Endpoint)
}

func TestCreateReceivers(t *testing.T) {
	f := NewFactory()
	params := receivertest.NewNopSettings(f.Type())

	tr, err := f.CreateTraces(context.Background(), params, f.CreateDefaultConfig(), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tr)

	mr, err := f.CreateMetrics(context.Background(), params, f.CreateDefaultConfig(), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, mr)

	lr, err := f.CreateLogs(context.Background(), params, f.CreateDefaultConfig(), consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, lr)
}

func TestConfigValidate(t *testing.T) {
	c := &Config{Endpoint: "0.0.0.0:4320"}
	require.NoError(t, c.Validate())

	c.Endpoint = ""
	require.Error(t, c.Validate())

	c.Endpoint = "0.0.0.0:4320"
	c.NumEventLoops = -1
	require.Error(t, c.Validate())
}
