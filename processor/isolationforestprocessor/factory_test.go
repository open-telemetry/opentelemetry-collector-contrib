// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// factory_test.go - Factory wiring & creation tests
package isolationforestprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestFactory_Type_And_DefaultConfig(t *testing.T) {
	factory := NewFactory()

	// Ensure the registered type matches what we expect.
	wantType := component.MustNewType("isolationforest")
	assert.Equal(t, wantType, factory.Type())

	// Default config should be non-nil and of our concrete *Config type.
	rawCfg := factory.CreateDefaultConfig()
	require.NotNil(t, rawCfg)
	_, ok := rawCfg.(*Config)
	require.True(t, ok, "CreateDefaultConfig should return *Config")
}

func TestFactory_CreateTraces(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()

	// NewNopSettings() has NO args in current API.
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	next := consumertest.NewNop()
	p, err := factory.CreateTraces(context.Background(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestFactory_CreateMetrics(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	next := consumertest.NewNop()
	p, err := factory.CreateMetrics(context.Background(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, p.Shutdown(context.Background()))
}

func TestFactory_CreateLogs(t *testing.T) {
	factory := NewFactory()
	rawCfg := factory.CreateDefaultConfig()
	settings := processortest.NewNopSettings(component.MustNewType("isolationforest"))

	next := consumertest.NewNop()
	p, err := factory.CreateLogs(context.Background(), settings, rawCfg, next)
	require.NoError(t, err)
	require.NotNil(t, p)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, p.Shutdown(context.Background()))
}
