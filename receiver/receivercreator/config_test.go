// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal/metadata"
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
	case component.KindConnector:
		return mh.factories.Connectors[componentType]
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
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id:       component.MustNewIDWithName("receiver_creator", ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				receiverTemplates: map[string]receiverTemplate{
					"examplereceiver/1": {
						receiverConfig: receiverConfig{
							id: component.MustNewIDWithName("examplereceiver", "1"),
							config: userConfigMap{
								"key": "value",
							},
							endpointID: "endpoint.id",
						},
						Rule:               `type == "port"`,
						ResourceAttributes: map[string]any{"one": "two"},
						rule:               portRule,
						signals:            receiverSignals{metrics: true, logs: true, traces: true},
					},
					"nop/1": {
						receiverConfig: receiverConfig{
							id: component.MustNewIDWithName("nop", "1"),
							config: userConfigMap{
								endpointConfigKey: "localhost:12345",
							},
							endpointID: "endpoint.id",
						},
						Rule:               `type == "port"`,
						ResourceAttributes: map[string]any{"two": "three"},
						rule:               portRule,
						signals:            receiverSignals{metrics: true, logs: true, traces: true},
					},
				},
				WatchObservers: []component.ID{
					component.MustNewID("mock_observer"),
					component.MustNewIDWithName("mock_observer", "with_name"),
				},
				ResourceAttributes: map[observer.EndpointType]map[string]string{
					observer.ContainerType:    {"container.key": "container.value"},
					observer.PodType:          {"pod.key": "pod.value"},
					observer.PodContainerType: {"pod.container.key": "pod.container.value"},
					observer.PortType:         {"port.key": "port.value"},
					observer.HostPortType:     {"hostport.key": "hostport.value"},
					observer.K8sServiceType:   {"k8s.service.key": "k8s.service.value"},
					observer.K8sIngressType:   {"k8s.ingress.key": "k8s.ingress.value"},
					observer.K8sNodeType:      {"k8s.node.key": "k8s.node.value"},
					observer.KafkaTopicType:   {},
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestInvalidResourceAttributeEndpointType(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factories.Receivers[component.MustNewType("nop")] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "invalid-resource-attributes.yaml"), factories)
	require.ErrorContains(t, err, "error reading configuration for \"receiver_creator\": resource attributes for unsupported endpoint type \"not.a.real.type\"")
	require.Nil(t, cfg)
}

func TestInvalidReceiverResourceAttributeValueType(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factories.Receivers[component.MustNewType("nop")] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "invalid-receiver-resource-attributes.yaml"), factories)
	require.ErrorContains(t, err, "error reading configuration for \"receiver_creator\": unsupported `resource_attributes` \"one\" value <nil> in examplereceiver/1")
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
	consumer.Logs
	consumer.Metrics
	consumer.Traces
	rcvr.Settings
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

func (*nopWithEndpointFactory) CreateLogs(
	_ context.Context,
	rcs rcvr.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (rcvr.Logs, error) {
	return &nopWithEndpointReceiver{
		Logs:     nextConsumer,
		Settings: rcs,
		cfg:      cfg,
	}, nil
}

func (*nopWithEndpointFactory) CreateMetrics(
	_ context.Context,
	rcs rcvr.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (rcvr.Metrics, error) {
	return &nopWithEndpointReceiver{
		Metrics:  nextConsumer,
		Settings: rcs,
		cfg:      cfg,
	}, nil
}

func (*nopWithEndpointFactory) CreateTraces(
	_ context.Context,
	rcs rcvr.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (rcvr.Traces, error) {
	return &nopWithEndpointReceiver{
		Traces:   nextConsumer,
		Settings: rcs,
		cfg:      cfg,
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
	consumer.Logs
	consumer.Metrics
	consumer.Traces
	rcvr.Settings
	cfg component.Config
}

func (*nopWithoutEndpointFactory) CreateDefaultConfig() component.Config {
	return &nopWithoutEndpointConfig{
		IntField: 2345,
	}
}

func (*nopWithoutEndpointFactory) CreateLogs(
	_ context.Context,
	rcs rcvr.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (rcvr.Logs, error) {
	return &nopWithoutEndpointReceiver{
		Logs:     nextConsumer,
		Settings: rcs,
		cfg:      cfg,
	}, nil
}

func (*nopWithoutEndpointFactory) CreateMetrics(
	_ context.Context,
	rcs rcvr.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (rcvr.Metrics, error) {
	return &nopWithoutEndpointReceiver{
		Metrics:  nextConsumer,
		Settings: rcs,
		cfg:      cfg,
	}, nil
}

func (*nopWithoutEndpointFactory) CreateTraces(
	_ context.Context,
	rcs rcvr.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (rcvr.Traces, error) {
	return &nopWithoutEndpointReceiver{
		Traces:   nextConsumer,
		Settings: rcs,
		cfg:      cfg,
	}, nil
}

type nopWithFilelogConfig struct {
	Include         []string `mapstructure:"include"`
	IncludeFileName bool     `mapstructure:"include_file_name"`
	IncludeFilePath bool     `mapstructure:"include_file_path"`
	Operators       []any    `mapstructure:"operators"`
}

type nopWithFilelogFactory struct {
	rcvr.Factory
}

type nopWithFilelogReceiver struct {
	mockComponent
	consumer.Logs
	consumer.Metrics
	consumer.Traces
	rcvr.Settings
	cfg component.Config
}

func (*nopWithFilelogFactory) CreateDefaultConfig() component.Config {
	return &nopWithFilelogConfig{}
}

func (*nopWithFilelogFactory) CreateLogs(
	_ context.Context,
	rcs rcvr.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (rcvr.Logs, error) {
	return &nopWithEndpointReceiver{
		Logs:     nextConsumer,
		Settings: rcs,
		cfg:      cfg,
	}, nil
}
