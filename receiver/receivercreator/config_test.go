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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
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

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       config.ComponentID
		expected config.Receiver
	}{
		{
			id:       config.NewComponentIDWithName(typeStr, ""),
			expected: createDefaultConfig(),
		},
		{
			id: config.NewComponentIDWithName(typeStr, "1"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				receiverTemplates: map[string]receiverTemplate{
					"examplereceiver/1": {
						receiverConfig: receiverConfig{
							id: config.NewComponentIDWithName("examplereceiver", "1"),
							config: userConfigMap{
								"key": "value",
							},
							endpointID: "endpoint.id",
						},
						Rule:               `type == "port"`,
						ResourceAttributes: map[string]interface{}{"one": "two"},
						rule:               newRuleOrPanic(`type == "port"`),
					},
					"nop/1": {
						receiverConfig: receiverConfig{
							id: config.NewComponentIDWithName("nop", "1"),
							config: userConfigMap{
								endpointConfigKey: "localhost:12345",
							},
							endpointID: "endpoint.id",
						},
						Rule:               `type == "port"`,
						ResourceAttributes: map[string]interface{}{"two": "three"},
						rule:               newRuleOrPanic(`type == "port"`),
					},
				},
				WatchObservers: []config.ComponentID{
					config.NewComponentID("mock_observer"),
					config.NewComponentIDWithName("mock_observer", "with_name"),
				},
				ResourceAttributes: map[observer.EndpointType]map[string]string{
					observer.ContainerType: {"container.key": "container.value"},
					observer.PodType:       {"pod.key": "pod.value"},
					observer.PortType:      {"port.key": "port.value"},
					observer.HostPortType:  {"hostport.key": "hostport.value"},
					observer.K8sNodeType:   {"k8s.node.key": "k8s.node.value"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestInvalidResourceAttributeEndpointType(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.Nil(t, err)

	factories.Receivers[("nop")] = &nopWithEndpointFactory{ReceiverFactory: componenttest.NewNopReceiverFactory()}

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid-resource-attributes.yaml"), factories)
	require.Contains(t, err.Error(), "error reading receivers configuration for \"receiver_creator\": resource attributes for unsupported endpoint type \"not.a.real.type\"")
	require.Nil(t, cfg)
}

func TestInvalidReceiverResourceAttributeValueType(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.Nil(t, err)

	factories.Receivers[("nop")] = &nopWithEndpointFactory{ReceiverFactory: componenttest.NewNopReceiverFactory()}

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "invalid-receiver-resource-attributes.yaml"), factories)
	require.Contains(t, err.Error(), "error reading receivers configuration for \"receiver_creator\": unsupported `resource_attributes` \"one\" value <nil> in examplereceiver/1")
	require.Nil(t, cfg)
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
	component.ReceiverCreateSettings
}

func (*nopWithEndpointFactory) CreateDefaultConfig() config.Receiver {
	return &nopWithEndpointConfig{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID("nop")),
	}
}

type mockComponent struct {
	component.StartFunc
	component.ShutdownFunc
}

func (*nopWithEndpointFactory) CreateMetricsReceiver(
	ctx context.Context,
	rcs component.ReceiverCreateSettings,
	_ config.Receiver,
	nextConsumer consumer.Metrics) (component.MetricsReceiver, error) {
	return &nopWithEndpointReceiver{
		Component:              mockComponent{},
		Metrics:                nextConsumer,
		ReceiverCreateSettings: rcs,
	}, nil
}
