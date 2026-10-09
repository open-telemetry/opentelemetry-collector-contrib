// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig()
	require.NotNil(t, cfg)
	assert.NoError(t, confmap.Validate(cfg))
}

func TestCreateReceivers(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings(factory.Type())
	host := componenttest.NewNopHost()

	lr, err := factory.CreateLogs(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, lr.Start(t.Context(), host))
	require.NoError(t, lr.Shutdown(t.Context()))

	mr, err := factory.CreateMetrics(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, mr.Start(t.Context(), host))
	require.NoError(t, mr.Shutdown(t.Context()))

	tr, err := factory.CreateTraces(t.Context(), set, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NoError(t, tr.Start(t.Context(), host))
	require.NoError(t, tr.Shutdown(t.Context()))
}

func TestCreateReceiversShareInstance(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	set := receivertest.NewNopSettings(factory.Type())
	logs, metrics, traces := consumertest.NewNop(), consumertest.NewNop(), consumertest.NewNop()

	lr, err := factory.CreateLogs(t.Context(), set, cfg, logs)
	require.NoError(t, err)
	mr, err := factory.CreateMetrics(t.Context(), set, cfg, metrics)
	require.NoError(t, err)
	tr, err := factory.CreateTraces(t.Context(), set, cfg, traces)
	require.NoError(t, err)
	assert.Same(t, lr, mr)
	assert.Same(t, lr, tr)

	// Each signal's consumer is registered on the shared instance.
	r := getOrCreateReceiver(set, cfg).Unwrap().(*natsReceiver)
	assert.Same(t, logs, r.nextLogs)
	assert.Same(t, metrics, r.nextMetrics)
	assert.Same(t, traces, r.nextTraces)

	// A different config gets its own instance.
	otherSC := getOrCreateReceiver(set, factory.CreateDefaultConfig())
	assert.NotSame(t, r, otherSC.Unwrap())
	require.NoError(t, otherSC.Shutdown(t.Context()))

	// Shutdown releases the cached instance.
	require.NoError(t, lr.Shutdown(t.Context()))
	assert.NotSame(t, r, getOrCreateReceiver(set, cfg).Unwrap())
}
