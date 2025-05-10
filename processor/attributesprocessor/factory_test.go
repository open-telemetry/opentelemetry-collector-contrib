// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor/internal/metadata"
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
	oCfg := cfg.(*Config)
	// Missing key
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "", Value: 123, Action: attraction.UPSERT},
	}
	ap, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
	// Invalid target type
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "http.status_code", ConvertedType: "array", Action: attraction.CONVERT},
	}
	ap2, err2 := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err2)
	assert.Equal(t, "error creating AttrProc due to invalid value \"array\" in field \"converted_type\" for action \"convert\" at the 0-th action", err2.Error())
	assert.Nil(t, ap2)
}

func TestFactoryCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "a key", Action: attraction.DELETE},
	}

	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	oCfg.Actions = []attraction.ActionKeyValue{
		{Action: attraction.DELETE},
	}
	tp, err = factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Nil(t, tp)
	assert.Error(t, err)
}

func TestFactory_CreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Actions = []attraction.ActionKeyValue{
		{Key: "fake_key", Action: attraction.INSERT, Value: "100"},
	}

	mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	require.NotNil(t, mp)
	require.NoError(t, err)

	cfg.(*Config).Actions = []attraction.ActionKeyValue{
		{Key: "fake_key", Action: attraction.UPSERT},
	}

	// Upsert should fail on non-existent key
	mp, err = factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	require.Nil(t, mp)
	require.Error(t, err)
}

func TestFactoryCreateLogs_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	// Missing key
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "", Value: 123, Action: attraction.UPSERT},
	}
	ap, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "a key", Action: attraction.DELETE},
	}

	tp, err := factory.CreateLogs(
		context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	oCfg.Actions = []attraction.ActionKeyValue{
		{Action: attraction.DELETE},
	}
	tp, err = factory.CreateLogs(
		context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Nil(t, tp)
	assert.Error(t, err)
}
