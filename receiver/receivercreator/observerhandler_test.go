// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestOnAddForMetrics(t *testing.T) {
	for _, test := range []struct {
		name                   string
		receiverTemplateID     component.ID
		receiverTemplateConfig userConfigMap
		expectedReceiverType   component.Component
		expectedReceiverConfig component.Config
		expectedError          string
	}{
		{
			name:                   "dynamically set with supported endpoint",
			receiverTemplateID:     component.NewIDWithName("with.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 12345678},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 12345678,
				Endpoint: "localhost:1234",
			},
		},
		{
			name:                   "inherits supported endpoint",
			receiverTemplateID:     component.NewIDWithName("with.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "some.endpoint"},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 1234,
				Endpoint: "some.endpoint",
			},
		},
		{
			name:                   "not dynamically set with unsupported endpoint",
			receiverTemplateID:     component.NewIDWithName("without.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 23456789, "not_endpoint": "not.an.endpoint"},
			expectedReceiverType:   &nopWithoutEndpointReceiver{},
			expectedReceiverConfig: &nopWithoutEndpointConfig{
				IntField:    23456789,
				NotEndpoint: "not.an.endpoint",
			},
		},
		{
			name:                   "inherits unsupported endpoint",
			receiverTemplateID:     component.NewIDWithName("without.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "unsupported.endpoint"},
			expectedError:          "failed to load \"without.endpoint/some.name\" template config: 1 error(s) decoding:\n\n* '' has invalid keys: endpoint",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			rcvrCfg := receiverConfig{
				id:         test.receiverTemplateID,
				config:     test.receiverTemplateConfig,
				endpointID: portEndpoint.ID,
			}
			cfg.receiverTemplates = map[string]receiverTemplate{
				rcvrCfg.id.String(): {
					receiverConfig:     rcvrCfg,
					rule:               portRule,
					Rule:               `type == "port"`,
					ResourceAttributes: map[string]any{},
				},
			}

			handler, mr := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil)
			handler.OnAdd([]observer.Endpoint{
				portEndpoint,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.EqualError(t, mr.lastError, test.expectedError)
				require.Nil(t, mr.startedComponent)
				return
			}

			assert.Equal(t, 1, handler.receiversByEndpointID.Size())
			require.NoError(t, mr.lastError)
			require.NotNil(t, mr.startedComponent)

			wr, ok := mr.startedComponent.(*wrappedReceiver)
			require.True(t, ok)

			require.Nil(t, wr.logs)
			require.Nil(t, wr.traces)

			var actualConfig component.Config
			switch v := wr.metrics.(type) {
			case *nopWithEndpointReceiver:
				require.NotNil(t, v)
				actualConfig = v.cfg
			case *nopWithoutEndpointReceiver:
				require.NotNil(t, v)
				actualConfig = v.cfg
			default:
				t.Fatalf("unexpected startedComponent: %T", v)
			}
			require.Equal(t, test.expectedReceiverConfig, actualConfig)
		})
	}
}

func TestOnAddForLogs(t *testing.T) {
	for _, test := range []struct {
		name                   string
		receiverTemplateID     component.ID
		receiverTemplateConfig userConfigMap
		expectedReceiverType   component.Component
		expectedReceiverConfig component.Config
		expectedError          string
	}{
		{
			name:                   "dynamically set with supported endpoint",
			receiverTemplateID:     component.NewIDWithName("with.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 12345678},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 12345678,
				Endpoint: "localhost:1234",
			},
		},
		{
			name:                   "inherits supported endpoint",
			receiverTemplateID:     component.NewIDWithName("with.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "some.endpoint"},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 1234,
				Endpoint: "some.endpoint",
			},
		},
		{
			name:                   "not dynamically set with unsupported endpoint",
			receiverTemplateID:     component.NewIDWithName("without.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 23456789, "not_endpoint": "not.an.endpoint"},
			expectedReceiverType:   &nopWithoutEndpointReceiver{},
			expectedReceiverConfig: &nopWithoutEndpointConfig{
				IntField:    23456789,
				NotEndpoint: "not.an.endpoint",
			},
		},
		{
			name:                   "inherits unsupported endpoint",
			receiverTemplateID:     component.NewIDWithName("without.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "unsupported.endpoint"},
			expectedError:          "failed to load \"without.endpoint/some.name\" template config: 1 error(s) decoding:\n\n* '' has invalid keys: endpoint",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			rcvrCfg := receiverConfig{
				id:         test.receiverTemplateID,
				config:     test.receiverTemplateConfig,
				endpointID: portEndpoint.ID,
			}
			cfg.receiverTemplates = map[string]receiverTemplate{
				rcvrCfg.id.String(): {
					receiverConfig:     rcvrCfg,
					rule:               portRule,
					Rule:               `type == "port"`,
					ResourceAttributes: map[string]any{},
				},
			}

			handler, mr := newObserverHandler(t, cfg, consumertest.NewNop(), nil, nil)
			handler.OnAdd([]observer.Endpoint{
				portEndpoint,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.EqualError(t, mr.lastError, test.expectedError)
				require.Nil(t, mr.startedComponent)
				return
			}

			assert.Equal(t, 1, handler.receiversByEndpointID.Size())
			require.NoError(t, mr.lastError)
			require.NotNil(t, mr.startedComponent)

			wr, ok := mr.startedComponent.(*wrappedReceiver)
			require.True(t, ok)

			require.Nil(t, wr.metrics)
			require.Nil(t, wr.traces)

			var actualConfig component.Config
			switch v := wr.logs.(type) {
			case *nopWithEndpointReceiver:
				require.NotNil(t, v)
				actualConfig = v.cfg
			case *nopWithoutEndpointReceiver:
				require.NotNil(t, v)
				actualConfig = v.cfg
			default:
				t.Fatalf("unexpected startedComponent: %T", v)
			}
			require.Equal(t, test.expectedReceiverConfig, actualConfig)
		})
	}
}

func TestOnAddForTraces(t *testing.T) {
	for _, test := range []struct {
		name                   string
		receiverTemplateID     component.ID
		receiverTemplateConfig userConfigMap
		expectedReceiverType   component.Component
		expectedReceiverConfig component.Config
		expectedError          string
	}{
		{
			name:                   "dynamically set with supported endpoint",
			receiverTemplateID:     component.NewIDWithName("with.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 12345678},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 12345678,
				Endpoint: "localhost:1234",
			},
		},
		{
			name:                   "inherits supported endpoint",
			receiverTemplateID:     component.NewIDWithName("with.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "some.endpoint"},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 1234,
				Endpoint: "some.endpoint",
			},
		},
		{
			name:                   "not dynamically set with unsupported endpoint",
			receiverTemplateID:     component.NewIDWithName("without.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 23456789, "not_endpoint": "not.an.endpoint"},
			expectedReceiverType:   &nopWithoutEndpointReceiver{},
			expectedReceiverConfig: &nopWithoutEndpointConfig{
				IntField:    23456789,
				NotEndpoint: "not.an.endpoint",
			},
		},
		{
			name:                   "inherits unsupported endpoint",
			receiverTemplateID:     component.NewIDWithName("without.endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "unsupported.endpoint"},
			expectedError:          "failed to load \"without.endpoint/some.name\" template config: 1 error(s) decoding:\n\n* '' has invalid keys: endpoint",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			rcvrCfg := receiverConfig{
				id:         test.receiverTemplateID,
				config:     test.receiverTemplateConfig,
				endpointID: portEndpoint.ID,
			}
			cfg.receiverTemplates = map[string]receiverTemplate{
				rcvrCfg.id.String(): {
					receiverConfig:     rcvrCfg,
					rule:               portRule,
					Rule:               `type == "port"`,
					ResourceAttributes: map[string]any{},
				},
			}

			handler, mr := newObserverHandler(t, cfg, nil, nil, consumertest.NewNop())
			handler.OnAdd([]observer.Endpoint{
				portEndpoint,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.EqualError(t, mr.lastError, test.expectedError)
				require.Nil(t, mr.startedComponent)
				return
			}

			assert.Equal(t, 1, handler.receiversByEndpointID.Size())
			require.NoError(t, mr.lastError)
			require.NotNil(t, mr.startedComponent)

			wr, ok := mr.startedComponent.(*wrappedReceiver)
			require.True(t, ok)

			require.Nil(t, wr.logs)
			require.Nil(t, wr.metrics)

			var actualConfig component.Config
			switch v := wr.traces.(type) {
			case *nopWithEndpointReceiver:
				require.NotNil(t, v)
				actualConfig = v.cfg
			case *nopWithoutEndpointReceiver:
				require.NotNil(t, v)
				actualConfig = v.cfg
			default:
				t.Fatalf("unexpected startedComponent: %T", v)
			}
			require.Equal(t, test.expectedReceiverConfig, actualConfig)

		})
	}
}

func TestOnRemoveForMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rcvrCfg := receiverConfig{
		id:         component.NewIDWithName("with.endpoint", "some.name"),
		config:     userConfigMap{"endpoint": "some.endpoint"},
		endpointID: portEndpoint.ID,
	}
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               portRule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
		},
	}
	handler, r := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	rcvr := r.startedComponent
	require.NotNil(t, rcvr)
	require.NoError(t, r.lastError)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	assert.Equal(t, 0, handler.receiversByEndpointID.Size())
	require.Same(t, rcvr, r.shutdownComponent)
	require.NoError(t, r.lastError)
}

func TestOnRemoveForLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rcvrCfg := receiverConfig{
		id:         component.NewIDWithName("with.endpoint", "some.name"),
		config:     userConfigMap{"endpoint": "some.endpoint"},
		endpointID: portEndpoint.ID,
	}
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               portRule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
		},
	}
	handler, r := newObserverHandler(t, cfg, consumertest.NewNop(), nil, nil)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	rcvr := r.startedComponent
	require.NotNil(t, rcvr)
	require.NoError(t, r.lastError)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	assert.Equal(t, 0, handler.receiversByEndpointID.Size())
	require.Same(t, rcvr, r.shutdownComponent)
	require.NoError(t, r.lastError)
}

func TestOnChange(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rcvrCfg := receiverConfig{
		id:         component.NewIDWithName("with.endpoint", "some.name"),
		config:     userConfigMap{"endpoint": "some.endpoint"},
		endpointID: portEndpoint.ID,
	}
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               portRule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
		},
	}
	handler, r := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	origRcvr := r.startedComponent
	require.NotNil(t, origRcvr)
	require.NoError(t, r.lastError)

	handler.OnChange([]observer.Endpoint{portEndpoint})

	require.NoError(t, r.lastError)
	assert.Same(t, origRcvr, r.shutdownComponent)

	newRcvr := r.startedComponent
	require.NotSame(t, origRcvr, newRcvr)
	require.NotNil(t, newRcvr)

	assert.Equal(t, 1, handler.receiversByEndpointID.Size())
	assert.Same(t, newRcvr, handler.receiversByEndpointID.Get("port-1")[0])
}

type mockRunner struct {
	receiverRunner
	startedComponent  component.Component
	shutdownComponent component.Component
	lastError         error
}

func (r *mockRunner) start(
	receiver receiverConfig,
	discoveredConfig userConfigMap,
	consumer *enhancingConsumer,
) (component.Component, error) {
	r.startedComponent, r.lastError = r.receiverRunner.start(receiver, discoveredConfig, consumer)
	return r.startedComponent, r.lastError
}

func (r *mockRunner) shutdown(rcvr component.Component) error {
	r.shutdownComponent = rcvr
	r.lastError = r.receiverRunner.shutdown(rcvr)
	return r.lastError
}

var _ component.Host = (*mockHost)(nil)

type mockHost struct {
	t         *testing.T
	factories otelcol.Factories
}

func newMockHost(t *testing.T) *mockHost {
	factories, err := otelcoltest.NopFactories()
	require.Nil(t, err)
	factories.Receivers["with.endpoint"] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}
	factories.Receivers["without.endpoint"] = &nopWithoutEndpointFactory{Factory: receivertest.NewNopFactory()}
	return &mockHost{t: t, factories: factories}
}

func (m *mockHost) ReportFatalError(err error) {
	m.t.Fatal("ReportFatalError", err)
}

func (m *mockHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	require.Equal(m.t, component.KindReceiver, kind, "mockhost can only retrieve receiver factories")
	return m.factories.Receivers[componentType]
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	m.t.Fatal("GetExtensions")
	return nil
}

func (m *mockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	m.t.Fatal("GetExporters")
	return nil
}

func newMockRunner(t *testing.T) *mockRunner {
	return &mockRunner{
		receiverRunner: receiverRunner{
			params:      receivertest.NewNopCreateSettings(),
			idNamespace: component.NewIDWithName("some.type", "some.name"),
			host:        newMockHost(t),
		},
	}
}

func newObserverHandler(
	t *testing.T, config *Config,
	nextLogs consumer.Logs,
	nextMetrics consumer.Metrics,
	nextTraces consumer.Traces,
) (*observerHandler, *mockRunner) {
	set := receivertest.NewNopCreateSettings()
	set.ID = component.NewIDWithName("some.type", "some.name")
	mr := newMockRunner(t)
	return &observerHandler{
		params:                set,
		config:                config,
		receiversByEndpointID: receiverMap{},
		runner:                mr,
		nextLogsConsumer:      nextLogs,
		nextMetricsConsumer:   nextMetrics,
		nextTracesConsumer:    nextTraces,
	}, mr
}
