// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	zapObserver "go.uber.org/zap/zaptest/observer"

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
					ResourceAttributes: map[string]interface{}{},
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
					ResourceAttributes: map[string]interface{}{},
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
					ResourceAttributes: map[string]interface{}{},
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
			ResourceAttributes: map[string]interface{}{},
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
			ResourceAttributes: map[string]interface{}{},
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
			ResourceAttributes: map[string]interface{}{},
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
	factories.Receivers["some.type"] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}
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
		logger:                set.TelemetrySettings.Logger,
	}, mr
}

func TestEndpointPropertiesDisregardedByDefault(t *testing.T) {
	endpoint := observer.Endpoint{
		ID:     "container.with.labels",
		Target: "some.target:2345",
		Details: &observer.Container{
			Labels: map[string]string{
				"io.opentelemetry.collector.receiver-creator.some.type/some.name.config.int_field":                         "234",
				"io.opentelemetry.collector.receiver-creator.some.type__some.name.config.float_field":                      "345.678",
				"io.opentelemetry.collector.receiver-creator.some.type/some.name.config.nested::bool_field":                "true",
				"io.opentelemetry.collector.receiver-creator.some.type__some.name.config.nested::doubly_nested::map_field": "{'one': true, 'two': null}",
				"io.opentelemetry.collector.receiver-creator.some.type/some.name.rule":                                     "type == \"container\" && labels['not.a.property'] == \"some.value\"",
				"io.opentelemetry.collector.receiver-creator.some.type__some.name.resource_attributes.attr.one":            "undesired.value.one",
				"io.opentelemetry.collector.receiver-creator.some.type/some.name.resource_attributes.attr.two":             "undesired.value.two",
				"not.a.property": "not.expected.value.for.rule",
			},
		},
	}

	containerRule := "type == \"container\""
	containerRuleExpr, err := newRule(containerRule)
	require.NoError(t, err)

	id := component.NewIDWithName("some.type", "some.name")
	cfg := createDefaultConfig().(*Config)
	cfg.receiverTemplates = map[string]receiverTemplate{
		id.String(): {
			receiverConfig: receiverConfig{id: id, config: userConfigMap{"int_field": 123}, endpointID: endpoint.ID},
			rule:           containerRuleExpr,
			Rule:           containerRule,
			ResourceAttributes: map[string]any{
				"attr.one": "user.config.value",
				"attr.two": "another.user.config.value",
			},
		},
	}
	sink := &consumertest.LogsSink{}
	handler, mr := newObserverHandler(t, cfg, sink, nil, nil)

	handler.OnAdd([]observer.Endpoint{endpoint})

	rcvr := mr.startedComponent
	assert.NotNil(t, rcvr)
	assert.NoError(t, mr.lastError)
	wr, ok := rcvr.(*wrappedReceiver)
	require.True(t, ok)
	r, ok := wr.logs.(*nopWithEndpointReceiver)
	require.True(t, ok)
	assert.Equal(t, &nopWithEndpointConfig{
		Endpoint: "some.target:2345",
		IntField: 123,
		Nested: nested{
			BoolField: false,
		},
	}, r.cfg)

	assert.Equal(t, 1, handler.receiversByEndpointID.Size())

	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	require.NoError(t, r.ConsumeLogs(context.Background(), ld))
	received := sink.AllLogs()
	require.Equal(t, 1, len(received))
	require.Equal(t, map[string]any{
		"attr.one": "user.config.value",
		"attr.two": "another.user.config.value",
	}, received[0].ResourceLogs().At(0).Resource().Attributes().AsRaw())
}

func TestReceiverAddedFromEndpointEnv(t *testing.T) {
	for _, endpoint := range []observer.Endpoint{
		{
			ID:     "container.with.labels",
			Target: "some.target:2345",
			Details: &observer.Container{
				Labels: map[string]string{
					"io.opentelemetry.collector.receiver-creator.some.type/some.name.config.int_field":                         "234",
					"io.opentelemetry.collector.receiver-creator.some.type__some.name.config.float_field":                      "345.678",
					"io.opentelemetry.collector.receiver-creator.some.type/some.name.config.nested::bool_field":                "true",
					"io.opentelemetry.collector.receiver-creator.some.type__some.name.config.nested::doubly_nested::map_field": "{'one': true, 'two': null}",
					"io.opentelemetry.collector.receiver-creator.some.type/some.name.rule":                                     "type == \"container\" && labels['not.a.property'] == \"some.value\"",
					"io.opentelemetry.collector.receiver-creator.some.type__some.name.resource_attributes.attr.one":            "value.one",
					"io.opentelemetry.collector.receiver-creator.some.type/some.name.resource_attributes.attr.two":             "value.two",
					"not.a.property": "some.value",
				},
			},
		},
		{
			ID:     "pod.with.annotations",
			Target: "another.target:3456",
			Details: &observer.Pod{
				Annotations: map[string]string{
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.int_field":                        "234",
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.float_field":                      "345.678",
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.endpoint":                         "some.target:2345",
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.nested::bool_field":               "true",
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.nested::doubly_nested::map_field": "{'one': true, 'two': null}",
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.rule":                                    "type == \"pod\" && annotations['not.a.property'] == \"some.value\"",
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.resource_attributes.attr.one":            "value.one",
					"receiver-creator.collector.opentelemetry.io/some.type__some.name.resource_attributes.attr.two":            "value.two",
					"not.a.property": "some.value",
				},
			},
		},
		{
			ID:     "port.with.annotations",
			Target: "another.target:3456",
			Details: &observer.Port{
				Pod: observer.Pod{
					Annotations: map[string]string{
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.int_field":                        "234",
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.float_field":                      "345.678",
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.endpoint":                         "some.target:2345",
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.nested::bool_field":               "true",
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.config.nested::doubly_nested::map_field": "{'one': true, 'two': null}",
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.rule":                                    "type == \"port\" && pod.annotations['not.a.property'] == \"some.value\"",
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.resource_attributes.attr.one":            "value.one",
						"receiver-creator.collector.opentelemetry.io/some.type__some.name.resource_attributes.attr.two":            "value.two",
						"not.a.property": "some.value",
					},
				},
			},
		},
		{
			ID:     "port.with.full.mapping.annotations",
			Target: "another.target:3456",
			Details: &observer.Port{
				Pod: observer.Pod{
					Annotations: map[string]string{
						"receiver-creator.collector.opentelemetry.io/some.type__some.name": `config:
  int_field: 234
  float_field: 345.678
  endpoint: some.target:2345
  nested:
    bool_field:               true
    doubly_nested:
      map_field:
        one: true
        two: null
rule: type == "port" && pod.annotations['not.a.property'] == "some.value"
resource_attributes:
  attr.one: value.one
  attr.two: value.two
`,
						"not.a.property": "some.value",
					},
				},
			},
		},
	} {
		t.Run(string(endpoint.ID), func(t *testing.T) {
			set := receivertest.NewNopCreateSettings()
			set.ID = component.NewIDWithName("some.type", "some.name")

			cfg := createDefaultConfig().(*Config)
			cfg.AcceptEndpointProperties = true
			cfg.receiverTemplates = map[string]receiverTemplate{
				set.ID.String(): {
					receiverConfig: receiverConfig{id: set.ID, config: userConfigMap{"int_field": 123}, endpointID: endpoint.ID},
					ResourceAttributes: map[string]any{
						"attr.one":   "undesired.value",
						"attr.two":   "another.undesired.value",
						"attr.three": "value.three",
					},
				},
			}
			sink := &consumertest.MetricsSink{}
			handler, mr := newObserverHandler(t, cfg, nil, sink, nil)
			handler.OnAdd([]observer.Endpoint{endpoint})

			rcvr := mr.startedComponent
			assert.NotNil(t, rcvr)
			assert.NoError(t, mr.lastError)
			wr, ok := rcvr.(*wrappedReceiver)
			require.True(t, ok)
			r, ok := wr.metrics.(*nopWithEndpointReceiver)
			require.True(t, ok)
			assert.Equal(t, &nopWithEndpointConfig{
				Endpoint:   "some.target:2345",
				IntField:   234,
				FloatField: 345.678,
				Nested: nested{
					BoolField: true,
					DoublyNested: doublyNested{
						MapField: map[string]any{
							"one": true, "two": nil,
						},
					},
				},
			}, r.cfg)

			assert.Equal(t, 1, handler.receiversByEndpointID.Size())

			md := pmetric.NewMetrics()
			md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			require.NoError(t, r.ConsumeMetrics(context.Background(), md))
			received := sink.AllMetrics()
			require.Equal(t, 1, len(received))
			require.Equal(t, map[string]any{
				"attr.one":   "value.one",
				"attr.two":   "value.two",
				"attr.three": "value.three",
			}, received[0].ResourceMetrics().At(0).Resource().Attributes().AsRaw())
		})
	}
}

func TestReceiverInvalidEndpointEnvContents(t *testing.T) {
	for _, tt := range []struct {
		endpoint                 observer.Endpoint
		name                     string
		expectedMessage          string
		expectedContext          map[string]any
		preventsReceiverCreation bool
	}{
		{
			name:                     "missing rule",
			expectedMessage:          "missing rule for property-created template",
			expectedContext:          map[string]any{"receiver": "some.type/some.name"},
			preventsReceiverCreation: true,
			endpoint: observer.Endpoint{
				ID:     "container.with.labels",
				Target: "some.target:2345",
				Details: &observer.Container{
					Labels: map[string]string{
						"io.opentelemetry.collector.receiver-creator.some.type/some.name.config.int_field": "123",
					},
				},
			},
		},
		{
			name:                     "invalid rule",
			expectedMessage:          "failed determining valid rule for property-created template",
			expectedContext:          map[string]any{"error": "rule must specify type", "receiver": "some.type/some.name"},
			preventsReceiverCreation: true,
			endpoint: observer.Endpoint{
				ID:     "container.with.labels",
				Target: "some.target:2345",
				Details: &observer.Container{
					Labels: map[string]string{
						"io.opentelemetry.collector.receiver-creator.some.type/some.name.config.int_field": "123",
						"io.opentelemetry.collector.receiver-creator.some.type/some.name.rule":             "invalid",
					},
				},
			},
		},
		{
			name:            "invalid yaml value",
			expectedMessage: "error creating property",
			expectedContext: map[string]any{
				"error":  "failed unmarshaling property (\"receiver-creator.collector.opentelemetry.io/some.type/some.name.config.nested::doubly_nested::map_field\") \"nested::doubly_nested::map_field: invalid: \" -> yaml: mapping values are not allowed in this context",
				"parsed": "receiver-creator.collector.opentelemetry.io/some.type/some.name.config.nested::doubly_nested::map_field",
			},
			preventsReceiverCreation: false,
			endpoint: observer.Endpoint{
				ID:     "pod.with.annotations",
				Target: "another.target:3456",
				Details: &observer.Pod{
					Annotations: map[string]string{
						"receiver-creator.collector.opentelemetry.io/some.type/some.name.config.nested::bool_field":               "false",
						"receiver-creator.collector.opentelemetry.io/some.type/some.name.config.nested::doubly_nested::map_field": "invalid: ",
						"receiver-creator.collector.opentelemetry.io/some.type/some.name.rule":                                    "type == \"pod\"",
						"receiver-creator.collector.opentelemetry.io/some.type/some.name.resource_attributes.attr.one":            "value.one",
						"receiver-creator.collector.opentelemetry.io/some.type/some.name.resource_attributes.attr.two":            "value.two",
						"not.a.property": "some.value",
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.AcceptEndpointProperties = true
			sink := &consumertest.MetricsSink{}
			handler, mr := newObserverHandler(t, cfg, nil, sink, nil)

			core, obs := zapObserver.New(zap.DebugLevel)
			logger := zap.New(core)

			handler.logger = logger
			handler.OnAdd([]observer.Endpoint{tt.endpoint})

			rcvr := mr.startedComponent
			if tt.preventsReceiverCreation {
				assert.Nil(t, rcvr)
				assert.NoError(t, mr.lastError)
			} else {
				assert.NotNil(t, rcvr)
				assert.NoError(t, mr.lastError)
			}

			require.True(t, func() bool {
				found := false
				for _, log := range obs.All() {
					if log.Message == tt.expectedMessage {
						found = true
						assert.Equal(t, tt.expectedContext, log.ContextMap())
						assert.Equal(t, zapcore.InfoLevel, log.Level)
						break
					}
				}
				return found
			}())
		})
	}
}
