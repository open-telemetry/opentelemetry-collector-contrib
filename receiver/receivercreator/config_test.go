// Copyright The OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type mockHostFactories struct {
	component.Host
	factories  otelcol.Factories
	extensions map[component.ID]component.Component
}

// GetFactory of the specified kind. Returns the factory for a component type.
func (mh *mockHostFactories) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
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

func (mh *mockHostFactories) GetExtensions() map[component.ID]component.Component {
	return mh.extensions
}

var portRule = func(s string) rule {
	r, err := newRule(s)
	if err != nil {
		panic(err)
	}
	return r
}(`type == "port"`)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(typeStr, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(typeStr, "1"),
			expected: &Config{
				receiverTemplates: map[string]receiverTemplate{
					"examplereceiver/1": {
						receiverConfig: receiverConfig{
							id: component.NewIDWithName("examplereceiver", "1"),
							config: userConfigMap{
								"key": "value",
							},
							endpointID: "endpoint.id",
						},
						Rule:               `type == "port"`,
						ResourceAttributes: map[string]interface{}{"one": "two"},
						rule:               portRule,
					},
					"nop/1": {
						receiverConfig: receiverConfig{
							id: component.NewIDWithName("nop", "1"),
							config: userConfigMap{
								endpointConfigKey: "localhost:12345",
							},
							endpointID: "endpoint.id",
						},
						Rule:               `type == "port"`,
						ResourceAttributes: map[string]interface{}{"two": "three"},
						rule:               portRule,
					},
				},
				WatchObservers: []component.ID{
					component.NewID("mock_observer"),
					component.NewIDWithName("mock_observer", "with_name"),
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestInvalidResourceAttributeEndpointType(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.Nil(t, err)

	factories.Receivers[("nop")] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "invalid-resource-attributes.yaml"), factories)
	require.Contains(t, err.Error(), "error reading configuration for \"receiver_creator\": resource attributes for unsupported endpoint type \"not.a.real.type\"")
	require.Nil(t, cfg)
}

func TestInvalidReceiverResourceAttributeValueType(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.Nil(t, err)

	factories.Receivers[("nop")] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "invalid-receiver-resource-attributes.yaml"), factories)
	require.Contains(t, err.Error(), "error reading configuration for \"receiver_creator\": unsupported `resource_attributes` \"one\" value <nil> in examplereceiver/1")
	require.Nil(t, cfg)
}

type nopWithEndpointConfig struct {
	Endpoint string `mapstructure:"endpoint"`
	IntField int    `mapstructure:"int_field"`
}

type nopWithEndpointFactory struct {
	rcvr.Factory
}

type nopWithEndpointReceiver struct {
	mockComponent
	consumer.Metrics
	rcvr.CreateSettings
	cfg component.Config
}

func (*nopWithEndpointFactory) CreateDefaultConfig() component.Config {
	return &nopWithEndpointConfig{
		IntField: 1234,
	}
}

type mockComponent struct {
	component.StartFunc
	component.ShutdownFunc
}

func (*nopWithEndpointFactory) CreateMetricsReceiver(
	_ context.Context,
	rcs rcvr.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics) (rcvr.Metrics, error) {
	return &nopWithEndpointReceiver{
		Metrics:        nextConsumer,
		CreateSettings: rcs,
		cfg:            cfg,
	}, nil
}

type nopWithoutEndpointConfig struct {
	NotEndpoint string `mapstructure:"not_endpoint"`
	IntField    int    `mapstructure:"int_field"`
}

type nopWithoutEndpointFactory struct {
	rcvr.Factory
}

type nopWithoutEndpointReceiver struct {
	mockComponent
	consumer.Metrics
	rcvr.CreateSettings
	cfg component.Config
}

func (*nopWithoutEndpointFactory) CreateDefaultConfig() component.Config {
	return &nopWithoutEndpointConfig{
		IntField: 2345,
	}
}

func (*nopWithoutEndpointFactory) CreateMetricsReceiver(
	_ context.Context,
	rcs rcvr.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics) (rcvr.Metrics, error) {
	return &nopWithoutEndpointReceiver{
		Metrics:        nextConsumer,
		CreateSettings: rcs,
		cfg:            cfg,
	}, nil
}
