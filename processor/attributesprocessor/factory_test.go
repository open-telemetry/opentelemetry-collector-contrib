// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	assert.Equal(t, factory.Type(), component.Type(metadata.Type))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{})
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestValidateConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Error(t, component.ValidateConfig(cfg))
}

func TestFactoryCreateTracesProcessor_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	// Missing key
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "", Value: 123, Action: attraction.UPSERT},
	}
	ap, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
	// Invalid target type
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "http.status_code", ConvertedType: "array", Action: attraction.CONVERT},
	}
	ap2, err2 := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err2)
	assert.Equal(t, "error creating AttrProc due to invalid value \"array\" in field \"converted_type\" for action \"convert\" at the 0-th action", err2.Error())
	assert.Nil(t, ap2)
}

func TestFactoryCreateTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "a key", Action: attraction.DELETE},
	}

	tp, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	tp, err = factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, nil)
	assert.Nil(t, tp)
	assert.Error(t, err)

	oCfg.Actions = []attraction.ActionKeyValue{
		{Action: attraction.DELETE},
	}
	tp, err = factory.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.Nil(t, tp)
	assert.Error(t, err)
}

func TestFactory_CreateMetricsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Actions = []attraction.ActionKeyValue{
		{Key: "fake_key", Action: attraction.INSERT, Value: "100"},
	}

	mp, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.NotNil(t, mp)
	require.NoError(t, err)

	mp, err = factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, nil)
	require.Nil(t, mp)
	require.Error(t, err)

	cfg.(*Config).Actions = []attraction.ActionKeyValue{
		{Key: "fake_key", Action: attraction.UPSERT},
	}

	// Upsert should fail on non-existent key
	mp, err = factory.CreateMetricsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	require.Nil(t, mp)
	require.Error(t, err)
}

func TestFactoryCreateLogsProcessor_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	// Missing key
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "", Value: 123, Action: attraction.UPSERT},
	}
	ap, err := factory.CreateLogsProcessor(context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateLogsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "a key", Action: attraction.DELETE},
	}

	tp, err := factory.CreateLogsProcessor(
		context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	tp, err = factory.CreateLogsProcessor(
		context.Background(), processortest.NewNopCreateSettings(), cfg, nil)
	assert.Nil(t, tp)
	assert.Error(t, err)

	oCfg.Actions = []attraction.ActionKeyValue{
		{Action: attraction.DELETE},
	}
	tp, err = factory.CreateLogsProcessor(
		context.Background(), processortest.NewNopCreateSettings(), cfg, consumertest.NewNop())
	assert.Nil(t, tp)
	assert.Error(t, err)
}
