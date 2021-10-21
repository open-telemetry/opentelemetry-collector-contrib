// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receivercreator

import (
	"context"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenthelper"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer"
)

type mockHostFactories struct {
	component.Host
	factories  component.Factories
	extensions map[config.ComponentID]component.Extension
}

// GetFactory of the specified kind. Returns the factory for a component type.
func (mh *mockHostFactories) GetFactory(kind component.Kind, componentType config.Type) component.Factory {
	switch kind {
	case component.KindReceiver:
		return mh.factories.Receivers[componentType]
	case component.KindProcessor:
		return mh.factories.Processors[componentType]
	case component.KindExporter:
		return mh.factories.Exporters[componentType]
	case component.KindExtension:
		return mh.factories.Extensions[componentType]
	}
	return nil
}

func (mh *mockHostFactories) GetExtensions() map[config.ComponentID]component.Extension {
	return mh.extensions
}

func exampleCreatorFactory(t *testing.T) (*mockHostFactories, *config.Config) {
	factories, err := componenttest.NopFactories()
	require.Nil(t, err)

	factories.Receivers[("nop")] = &nopWithEndpointFactory{ReceiverFactory: componenttest.NewNopReceiverFactory()}

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	return &mockHostFactories{Host: componenttest.NewNopHost(), factories: factories}, cfg
}

func TestLoadConfig(t *testing.T) {
	_, cfg := exampleCreatorFactory(t)
	factory := NewFactory()

	r0 := cfg.Receivers[config.NewComponentID("receiver_creator")]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Receivers[config.NewComponentIDWithName("receiver_creator", "1")].(*Config)

	assert.NotNil(t, r1)
	assert.Len(t, r1.receiverTemplates, 2)
	assert.Contains(t, r1.receiverTemplates, "examplereceiver/1")
	assert.Equal(t, `type == "port"`, r1.receiverTemplates["examplereceiver/1"].Rule)
	assert.Contains(t, r1.receiverTemplates, "nop/1")
	assert.Equal(t, `type == "port"`, r1.receiverTemplates["nop/1"].Rule)
	assert.Equal(t, userConfigMap{
		endpointConfigKey: "localhost:12345",
	}, r1.receiverTemplates["nop/1"].config)
	assert.Equal(t, []config.Type{"mock_observer"}, r1.WatchObservers)
}

type nopWithEndpointConfig struct {
	config.ReceiverSettings `mapstructure:",squash"`
	Endpoint                string `mapstructure:"endpoint"`
}

type nopWithEndpointFactory struct {
	component.ReceiverFactory
}

type nopWithEndpointReceiver struct {
	component.Component
	consumer.Metrics
}

func (*nopWithEndpointFactory) CreateDefaultConfig() config.Receiver {
	return &nopWithEndpointConfig{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID("nop")),
	}
}

func (*nopWithEndpointFactory) CreateMetricsReceiver(
	ctx context.Context,
	_ component.ReceiverCreateSettings,
	_ config.Receiver,
	nextConsumer consumer.Metrics) (component.MetricsReceiver, error) {
	return &nopWithEndpointReceiver{
		Component: componenthelper.New(),
		Metrics:   nextConsumer,
	}, nil
}
