// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator/internal/metadata"
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
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 12345678},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 12345678,
				Endpoint: "localhost:1234",
			},
		},
		{
			name:                   "inherits supported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "some.endpoint"},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 1234,
				Endpoint: "some.endpoint",
			},
		},
		{
			name:                   "not dynamically set with unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 23456789, "not_endpoint": "not.an.endpoint"},
			expectedReceiverType:   &nopWithoutEndpointReceiver{},
			expectedReceiverConfig: &nopWithoutEndpointConfig{
				IntField:    23456789,
				NotEndpoint: "not.an.endpoint",
			},
		},
		{
			name:                   "inherits unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "unsupported.endpoint"},
			expectedError:          "'receivercreator.nopWithoutEndpointConfig' has invalid keys: endpoint",
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
					signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
				},
			}

			handler, mr := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil, nil)
			handler.OnAdd([]observer.Endpoint{
				portEndpoint,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.ErrorContains(t, mr.lastError, test.expectedError)
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

func TestOnAddForMetricsWithHints(t *testing.T) {
	for _, test := range []struct {
		name                   string
		expectedReceiverType   component.Component
		expectedReceiverConfig component.Config
		expectedError          string
	}{
		{
			name:                 "dynamically set with supported endpoint",
			expectedReceiverType: &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 20,
				Endpoint: "1.2.3.4:6379",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Discovery.Enabled = true

			handler, mr := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil, nil)
			handler.OnAdd([]observer.Endpoint{
				portEndpointWithHints,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.ErrorContains(t, mr.lastError, test.expectedError)
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
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 12345678},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 12345678,
				Endpoint: "localhost:1234",
			},
		},
		{
			name:                   "inherits supported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "some.endpoint"},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 1234,
				Endpoint: "some.endpoint",
			},
		},
		{
			name:                   "not dynamically set with unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 23456789, "not_endpoint": "not.an.endpoint"},
			expectedReceiverType:   &nopWithoutEndpointReceiver{},
			expectedReceiverConfig: &nopWithoutEndpointConfig{
				IntField:    23456789,
				NotEndpoint: "not.an.endpoint",
			},
		},
		{
			name:                   "inherits unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "unsupported.endpoint"},
			expectedError:          "'receivercreator.nopWithoutEndpointConfig' has invalid keys: endpoint",
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
					signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
				},
			}

			handler, mr := newObserverHandler(t, cfg, consumertest.NewNop(), nil, nil, nil)
			handler.OnAdd([]observer.Endpoint{
				portEndpoint,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.ErrorContains(t, mr.lastError, test.expectedError)
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

func TestOnAddForLogsWithHints(t *testing.T) {
	for _, test := range []struct {
		name                   string
		expectedReceiverType   component.Component
		expectedReceiverConfig component.Config
		target                 observer.Endpoint
		hintsConfig            DiscoveryConfig
		expectedError          string
	}{
		{
			name:   "dynamically generated standard file_log receiver with explicit enablement",
			target: podContainerEndpointWithHints,
			hintsConfig: DiscoveryConfig{Enabled: true, DefaultFileLogConfig: userConfigMap{
				"include_file_path": true,
				"include_file_name": false,
				"operators":         []any{map[string]any{"id": "container-parser", "type": "container"}},
			}},
			expectedReceiverType: &nopWithFileLogReceiver{},
			expectedReceiverConfig: &nopWithFileLogConfig{
				Include:         []string{"/var/log/pods/default_pod-2_pod-2-UID/redis/*.log"},
				IncludeFileName: false,
				IncludeFilePath: true,
				Operators:       []any{map[string]any{"id": "container-parser", "type": "container"}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Discovery = test.hintsConfig

			handler, mr := newObserverHandler(t, cfg, consumertest.NewNop(), nil, nil, nil)
			handler.OnAdd([]observer.Endpoint{
				test.target,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.ErrorContains(t, mr.lastError, test.expectedError)
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
			case *nopWithFileLogReceiver:
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
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 12345678},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 12345678,
				Endpoint: "localhost:1234",
			},
		},
		{
			name:                   "inherits supported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "some.endpoint"},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 1234,
				Endpoint: "some.endpoint",
			},
		},
		{
			name:                   "not dynamically set with unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 23456789, "not_endpoint": "not.an.endpoint"},
			expectedReceiverType:   &nopWithoutEndpointReceiver{},
			expectedReceiverConfig: &nopWithoutEndpointConfig{
				IntField:    23456789,
				NotEndpoint: "not.an.endpoint",
			},
		},
		{
			name:                   "inherits unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "unsupported.endpoint"},
			expectedError:          "'receivercreator.nopWithoutEndpointConfig' has invalid keys: endpoint",
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
					signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
				},
			}

			handler, mr := newObserverHandler(t, cfg, nil, nil, consumertest.NewNop(), nil)
			handler.OnAdd([]observer.Endpoint{
				portEndpoint,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.ErrorContains(t, mr.lastError, test.expectedError)
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
		id:         component.MustNewIDWithName("with_endpoint", "some.name"),
		config:     userConfigMap{"endpoint": "some.endpoint"},
		endpointID: portEndpoint.ID,
	}
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               portRule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
		},
	}
	handler, r := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil, nil)
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
		id:         component.MustNewIDWithName("with_endpoint", "some.name"),
		config:     userConfigMap{"endpoint": "some.endpoint"},
		endpointID: portEndpoint.ID,
	}
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               portRule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
		},
	}
	handler, r := newObserverHandler(t, cfg, consumertest.NewNop(), nil, nil, nil)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	rcvr := r.startedComponent
	require.NotNil(t, rcvr)
	require.NoError(t, r.lastError)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	assert.Equal(t, 0, handler.receiversByEndpointID.Size())
	require.Same(t, rcvr, r.shutdownComponent)
	require.NoError(t, r.lastError)
}

func TestOnRemoveForTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rcvrCfg := receiverConfig{
		id:         component.MustNewIDWithName("with_endpoint", "some.name"),
		config:     userConfigMap{"endpoint": "some.endpoint"},
		endpointID: portEndpoint.ID,
	}
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               portRule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
		},
	}
	handler, r := newObserverHandler(t, cfg, nil, nil, consumertest.NewNop(), nil)
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	rcvr := r.startedComponent
	require.NotNil(t, rcvr)
	require.NoError(t, r.lastError)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	assert.Equal(t, 0, handler.receiversByEndpointID.Size())
	require.Same(t, rcvr, r.shutdownComponent)
	require.NoError(t, r.lastError)
}

func TestOnAddForProfiles(t *testing.T) {
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
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 12345678},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 12345678,
				Endpoint: "localhost:1234",
			},
		},
		{
			name:                   "inherits supported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("with_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "some.endpoint"},
			expectedReceiverType:   &nopWithEndpointReceiver{},
			expectedReceiverConfig: &nopWithEndpointConfig{
				IntField: 1234,
				Endpoint: "some.endpoint",
			},
		},
		{
			name:                   "not dynamically set with unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"int_field": 23456789, "not_endpoint": "not.an.endpoint"},
			expectedReceiverType:   &nopWithoutEndpointReceiver{},
			expectedReceiverConfig: &nopWithoutEndpointConfig{
				IntField:    23456789,
				NotEndpoint: "not.an.endpoint",
			},
		},
		{
			name:                   "inherits unsupported endpoint",
			receiverTemplateID:     component.MustNewIDWithName("without_endpoint", "some.name"),
			receiverTemplateConfig: userConfigMap{"endpoint": "unsupported.endpoint"},
			expectedError:          "'receivercreator.nopWithoutEndpointConfig' has invalid keys: endpoint",
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
					signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
				},
			}

			handler, mr := newObserverHandler(t, cfg, nil, nil, nil, &consumertest.ProfilesSink{})
			handler.OnAdd([]observer.Endpoint{
				portEndpoint,
				unsupportedEndpoint,
			})

			if test.expectedError != "" {
				assert.Equal(t, 0, handler.receiversByEndpointID.Size())
				require.Error(t, mr.lastError)
				require.ErrorContains(t, mr.lastError, test.expectedError)
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
			require.Nil(t, wr.traces)

			var actualConfig component.Config
			switch v := wr.profiles.(type) {
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

func TestOnRemoveForProfiles(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	rcvrCfg := receiverConfig{
		id:         component.MustNewIDWithName("with_endpoint", "some.name"),
		config:     userConfigMap{"endpoint": "some.endpoint"},
		endpointID: portEndpoint.ID,
	}
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               portRule,
			Rule:               `type == "port"`,
			ResourceAttributes: map[string]any{},
			signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
		},
	}
	handler, r := newObserverHandler(t, cfg, nil, nil, nil, &consumertest.ProfilesSink{})
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	rcvr := r.startedComponent
	require.NotNil(t, rcvr)
	require.NoError(t, r.lastError)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	assert.Equal(t, 0, handler.receiversByEndpointID.Size())
	require.Same(t, rcvr, r.shutdownComponent)
	require.NoError(t, r.lastError)
}

// changedPortEndpoint returns portEndpoint after a change to it.
func changedPortEndpoint(change func(e *observer.Endpoint, port *observer.Port)) observer.Endpoint {
	e := portEndpoint
	port := *portEndpoint.Details.(*observer.Port)
	port.Pod.Labels = map[string]string{}
	for k, v := range pod.Labels {
		port.Pod.Labels[k] = v
	}
	e.Details = &port
	change(&e, &port)
	return e
}

func newOnChangeHandler(t *testing.T, ruleStr string, config userConfigMap) (*observerHandler, *mockRunner) {
	cfg := createDefaultConfig().(*Config)
	rcvrCfg := receiverConfig{
		id:         component.MustNewIDWithName("with_endpoint", "some.name"),
		config:     config,
		endpointID: portEndpoint.ID,
	}
	rcvrRule, err := newRule(ruleStr)
	require.NoError(t, err)
	cfg.receiverTemplates = map[string]receiverTemplate{
		rcvrCfg.id.String(): {
			receiverConfig:     rcvrCfg,
			rule:               rcvrRule,
			Rule:               ruleStr,
			ResourceAttributes: map[string]any{"app": "`pod.labels[\"app\"]`"},
			signals:            receiverSignals{metrics: true, logs: true, traces: true, profiles: true},
		},
	}
	return newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil, nil)
}

func TestOnChange(t *testing.T) {
	const anyPort = `type == "port"`
	const redisPort = `type == "port" && pod.labels["app"] == "redis"`
	staticConfig := userConfigMap{"endpoint": "some.endpoint"}

	for _, test := range []struct {
		name    string
		rule    string
		config  userConfigMap
		change  func(e *observer.Endpoint, port *observer.Port)
		restart bool
		app     string
	}{
		{
			name:   "nothing the receiver uses changed",
			rule:   anyPort,
			config: staticConfig,
			change: func(_ *observer.Endpoint, port *observer.Port) { port.ContainerID = "container-id-2" },
			app:    "redis",
		},
		{
			name:   "only a resource attribute changed",
			rule:   anyPort,
			config: staticConfig,
			change: func(_ *observer.Endpoint, port *observer.Port) { port.Pod.Labels["app"] = "valkey" },
			app:    "valkey",
		},
		{
			name:    "the target changed and the config takes it by default",
			rule:    anyPort,
			config:  userConfigMap{},
			change:  func(e *observer.Endpoint, _ *observer.Port) { e.Target = "localhost:4321" },
			restart: true,
			app:     "redis",
		},
		{
			name:    "a value the config uses changed",
			rule:    anyPort,
			config:  userConfigMap{"endpoint": "`pod.labels[\"region\"]`:1234"},
			change:  func(_ *observer.Endpoint, port *observer.Port) { port.Pod.Labels["region"] = "east-1" },
			restart: true,
			app:     "redis",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler, r := newOnChangeHandler(t, test.rule, test.config)
			handler.OnAdd([]observer.Endpoint{portEndpoint})

			origRcvr := r.startedComponent
			require.NotNil(t, origRcvr)
			require.NoError(t, r.lastError)

			handler.OnChange([]observer.Endpoint{changedPortEndpoint(test.change)})
			require.NoError(t, r.lastError)

			if test.restart {
				assert.Same(t, origRcvr, r.shutdownComponent)
				require.NotSame(t, origRcvr, r.startedComponent)
			} else {
				assert.Nil(t, r.shutdownComponent)
				assert.Same(t, origRcvr, r.startedComponent)
			}

			assert.Equal(t, 1, handler.receiversByEndpointID.Size())
			assert.Same(t, r.startedComponent, handler.receiversByEndpointID.Get("port-1")[0])
			require.Len(t, handler.started["port-1"], 1)
			assert.Equal(t, test.app, handler.started["port-1"][0].consumer.attrs["app"])

			// A second change of the same kind is handled the same way, and removal stops the receiver.
			running := r.startedComponent
			handler.OnChange([]observer.Endpoint{changedPortEndpoint(test.change)})
			assert.Same(t, running, r.startedComponent)

			handler.OnRemove([]observer.Endpoint{portEndpoint})
			assert.Same(t, running, r.shutdownComponent)
			assert.Equal(t, 0, handler.receiversByEndpointID.Size())
			assert.Empty(t, handler.started)
		})
	}

	t.Run("the rule stops and starts matching", func(t *testing.T) {
		handler, r := newOnChangeHandler(t, redisPort, staticConfig)
		handler.OnAdd([]observer.Endpoint{portEndpoint})
		origRcvr := r.startedComponent
		require.NotNil(t, origRcvr)

		handler.OnChange([]observer.Endpoint{changedPortEndpoint(func(_ *observer.Endpoint, port *observer.Port) {
			port.Pod.Labels["app"] = "valkey"
		})})
		assert.Same(t, origRcvr, r.shutdownComponent)
		assert.Equal(t, 0, handler.receiversByEndpointID.Size())
		assert.Empty(t, handler.started)

		handler.OnChange([]observer.Endpoint{portEndpoint})
		require.NoError(t, r.lastError)
		require.NotSame(t, origRcvr, r.startedComponent)
		assert.Equal(t, 1, handler.receiversByEndpointID.Size())
	})
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

type mockHost struct {
	component.Host
	t         *testing.T
	factories otelcol.Factories
}

func newMockHost(t *testing.T, host component.Host) *mockHost {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)
	factories.Receivers[component.MustNewType("with_endpoint")] = &nopWithEndpointFactory{Factory: receivertest.NewNopFactory()}
	factories.Receivers[component.MustNewType("file_log")] = &nopWithFileLogFactory{Factory: receivertest.NewNopFactory()}
	factories.Receivers[component.MustNewType("without_endpoint")] = &nopWithoutEndpointFactory{Factory: receivertest.NewNopFactory()}
	return &mockHost{t: t, factories: factories, Host: host}
}

func (m *mockHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	require.Equal(m.t, component.KindReceiver, kind, "mockhost can only retrieve receiver factories")
	return m.factories.Receivers[componentType]
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	m.t.Fatal("GetExtensions")
	return nil
}

func newMockRunner(t *testing.T) *mockRunner {
	cs := receivertest.NewNopSettings(metadata.Type)
	return &mockRunner{
		receiverRunner: receiverRunner{
			logger:      cs.Logger,
			params:      cs,
			idNamespace: component.MustNewIDWithName("some_type", "some.name"),
			host: newMockHost(t, &reportingHost{
				reportFunc: func(event *componentstatus.Event) {
					require.NoError(t, event.Err())
				},
			}),
		},
	}
}

func newObserverHandler(
	t *testing.T, config *Config,
	nextLogs consumer.Logs,
	nextMetrics consumer.Metrics,
	nextTraces consumer.Traces,
	nextProfiles xconsumer.Profiles,
) (*observerHandler, *mockRunner) {
	set := receivertest.NewNopSettings(metadata.Type)
	set.ID = component.MustNewIDWithName("some_type", "some.name")
	mr := newMockRunner(t)
	return &observerHandler{
		params:                set,
		config:                config,
		receiversByEndpointID: receiverMap{},
		runner:                mr,
		nextLogsConsumer:      nextLogs,
		nextMetricsConsumer:   nextMetrics,
		nextTracesConsumer:    nextTraces,
		nextProfilesConsumer:  nextProfiles,
	}, mr
}

var _ componentstatus.Reporter = (*reportingHost)(nil)

type reportingHost struct {
	reportFunc func(event *componentstatus.Event)
}

func (*reportingHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *reportingHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
