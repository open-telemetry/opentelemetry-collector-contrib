// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sizeprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/multiplayer-app/opentelemetry-collector-contrib/processor/sizeprocessor/internal/metadata"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), metadata.Type)
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestValidateConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Error(t, component.ValidateConfig(cfg))
}

func TestFactoryCreateTraces_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	ap, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)

	ap2, err2 := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err2)
	assert.Equal(t, "error creating AttrProc due to invalid value \"array\" in field \"converted_type\" for action \"convert\" at the 0-th action", err2.Error())
	assert.Nil(t, ap2)
}

func TestFactoryCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	tp, err = factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Nil(t, tp)
	assert.Error(t, err)
}

func TestFactory_CreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NotNil(t, mp)
	require.NoError(t, err)

	// Upsert should fail on non-existent key
	mp, err = factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	require.Nil(t, mp)
	require.Error(t, err)
}

func TestFactoryCreateLogs_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	ap, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateLogs(
		context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	tp, err = factory.CreateLogs(
		context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Nil(t, tp)
	assert.Error(t, err)
}
