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
					signals:            receiverSignals{metrics: true, logs: true, traces: true},
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

			handler, mr := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil)
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
					signals:            receiverSignals{metrics: true, logs: true, traces: true},
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
			name:                 "dynamically generated standard filelog receiver with explicit enablement",
			target:               podContainerEndpointWithHints,
			hintsConfig:          DiscoveryConfig{Enabled: true},
			expectedReceiverType: &nopWithFilelogReceiver{},
			expectedReceiverConfig: &nopWithFilelogConfig{
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

			handler, mr := newObserverHandler(t, cfg, consumertest.NewNop(), nil, nil)
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
			case *nopWithFilelogReceiver:
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
					signals:            receiverSignals{metrics: true, logs: true, traces: true},
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
			signals:            receiverSignals{metrics: true, logs: true, traces: true},
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
			signals:            receiverSignals{metrics: true, logs: true, traces: true},
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
			signals:            receiverSignals{metrics: true, logs: true, traces: true},
		},
	}
	handler, r := newObserverHandler(t, cfg, nil, nil, consumertest.NewNop())
	handler.OnAdd([]observer.Endpoint{portEndpoint})

	rcvr := r.startedComponent
	require.NotNil(t, rcvr)
	require.NoError(t, r.lastError)

	handler.OnRemove([]observer.Endpoint{portEndpoint})

	assert.Equal(t, 0, handler.receiversByEndpointID.Size())
	require.Same(t, rcvr, r.shutdownComponent)
	require.NoError(t, r.lastError)
}

// TestOnChange verifies that OnChange only restarts receivers when their effective config changes.
//
// The endpoint "config" comes from two sources that are merged:
//   - User config: What the user specifies in their receiver template (may contain backtick expressions)
//   - Discovered config: Auto-populated values like endpoint target (when user doesn't specify them)
//
// A receiver should restart when EITHER source would produce different values.
func TestOnChange(t *testing.T) {
	tests := []struct {
		name               string
		templateConfig     userConfigMap
		resourceAttributes map[string]any // optional custom resource attributes for the template
		modifyEndpoint     func(e observer.Endpoint) observer.Endpoint // nil means no modification
		expectRestart      bool
	}{
		{
			name:           "static config unchanged - no restart",
			templateConfig: userConfigMap{"endpoint": "some.endpoint"}, // Static value, won't change
			modifyEndpoint: nil,                                        // Same endpoint
			expectRestart:  false,
		},
		{
			name:           "dynamic user config changed - restart",
			templateConfig: userConfigMap{"endpoint": "`endpoint`"}, // References endpoint target
			modifyEndpoint: func(e observer.Endpoint) observer.Endpoint {
				e.Target = "new.target:5678"
				return e
			},
			expectRestart: true,
		},
		{
			// When user doesn't specify "endpoint", it's auto-discovered from endpoint.Target.
			// If Target changes (e.g., pod IP change), receiver must restart.
			name:           "auto-discovered endpoint changed - restart",
			templateConfig: userConfigMap{"int_field": 12345}, // No endpoint - will be auto-discovered
			modifyEndpoint: func(e observer.Endpoint) observer.Endpoint {
				e.Target = "new.host:9999"
				return e
			},
			expectRestart: true,
		},
		{
			// When a custom resource attribute references a label via backtick expression,
			// and that label changes, the receiver should restart to pick up the new value.
			name:               "label change with custom resource attr - restart",
			templateConfig:     userConfigMap{"endpoint": "some.endpoint"},
			resourceAttributes: map[string]any{"app.label": "`pod.labels[\"app\"]`"},
			modifyEndpoint: func(e observer.Endpoint) observer.Endpoint {
				// Modify the pod labels in the endpoint details
				port := e.Details.(*observer.Port)
				newPod := port.Pod
				newPod.Labels = map[string]string{
					"app":    "redis-v2", // changed from "redis"
					"region": "west-1",
				}
				e.Details = &observer.Port{
					Name:           port.Name,
					Pod:            newPod,
					Port:           port.Port,
					Transport:      port.Transport,
					ContainerName:  port.ContainerName,
					ContainerID:    port.ContainerID,
					ContainerImage: port.ContainerImage,
				}
				return e
			},
			expectRestart: true,
		},
		{
			// When labels change but no resource attribute references them,
			// the receiver should NOT restart (config and default attrs unchanged).
			name:               "label change without resource attr reference - no restart",
			templateConfig:     userConfigMap{"endpoint": "some.endpoint"},
			resourceAttributes: map[string]any{}, // no custom attrs referencing labels
			modifyEndpoint: func(e observer.Endpoint) observer.Endpoint {
				port := e.Details.(*observer.Port)
				newPod := port.Pod
				newPod.Labels = map[string]string{
					"app":    "redis-v2", // changed, but not referenced
					"region": "west-1",
				}
				e.Details = &observer.Port{
					Name:           port.Name,
					Pod:            newPod,
					Port:           port.Port,
					Transport:      port.Transport,
					ContainerName:  port.ContainerName,
					ContainerID:    port.ContainerID,
					ContainerImage: port.ContainerImage,
				}
				return e
			},
			expectRestart: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: create handler and add initial endpoint
			cfg := createDefaultConfig().(*Config)
			rcvrCfg := receiverConfig{
				id:         component.MustNewIDWithName("with_endpoint", "some.name"),
				config:     tt.templateConfig,
				endpointID: portEndpoint.ID,
			}

			// Use custom resource attributes if provided, otherwise empty
			resAttrs := map[string]any{}
			if tt.resourceAttributes != nil {
				resAttrs = tt.resourceAttributes
			}

			cfg.receiverTemplates = map[string]receiverTemplate{
				rcvrCfg.id.String(): {
					receiverConfig:     rcvrCfg,
					rule:               portRule,
					Rule:               `type == "port"`,
					ResourceAttributes: resAttrs,
					signals:            receiverSignals{metrics: true, logs: true, traces: true},
				},
			}
			handler, r := newObserverHandler(t, cfg, nil, consumertest.NewNop(), nil)
			handler.OnAdd([]observer.Endpoint{portEndpoint})

			origRcvr := r.startedComponent
			require.NotNil(t, origRcvr)
			require.NoError(t, r.lastError)

			// Clear mock to track OnChange calls
			r.shutdownComponent = nil
			r.startedComponent = nil

			// Apply endpoint modification if specified
			endpoint := portEndpoint
			if tt.modifyEndpoint != nil {
				endpoint = tt.modifyEndpoint(portEndpoint)
			}

			// Trigger OnChange
			handler.OnChange([]observer.Endpoint{endpoint})
			require.NoError(t, r.lastError)

			// Verify restart behavior
			if tt.expectRestart {
				assert.Same(t, origRcvr, r.shutdownComponent, "original receiver should be shutdown")
				require.NotNil(t, r.startedComponent, "new receiver should be started")
				require.NotSame(t, origRcvr, r.startedComponent, "should be a different receiver instance")
			} else {
				assert.Nil(t, r.shutdownComponent, "receiver should not be shutdown")
				assert.Nil(t, r.startedComponent, "no new receiver should be started")
				assert.Same(t, origRcvr, handler.receiversByEndpointID.Get("port-1")[0].receiver)
			}
			assert.Equal(t, 1, handler.receiversByEndpointID.Size())
		})
	}
}

// TestResolveConfig verifies resolveConfig expands backtick expressions and properly
// separates user-specified config from auto-discovered config.
//
// The endpoint "config" comes from two sources:
//   - User config: What the user specifies in their template (may contain backtick expressions)
//   - Discovered config: Auto-populated when user doesn't set "endpoint" (uses endpoint.Target)
//
// The discovered config includes a marker flag (tmpSetEndpointConfigKey) so the runner
// knows to validate whether the receiver actually supports an "endpoint" field.
func TestResolveConfig(t *testing.T) {
	tests := []struct {
		name                     string
		templateConfig           userConfigMap
		env                      observer.EndpointEnv
		endpointTarget           string
		expectError              string // empty string means success expected
		expectUserEndpoint       string // expected endpoint in user config (empty if not expected)
		expectDiscoveredEndpoint string // expected endpoint in discovered config (empty if not expected)
		expectDiscoveredMarker   bool   // whether tmpSetEndpointConfigKey should be present
	}{
		{
			name:           "user template config expansion error",
			templateConfig: userConfigMap{"endpoint": "`(`"}, // Invalid expression syntax
			env:            observer.EndpointEnv{"type": "port"},
			endpointTarget: "localhost:1234",
			expectError:    "expanding user template config",
		},
		{
			name:           "discovered config expansion error",
			templateConfig: userConfigMap{}, // No endpoint, so Target will be used
			env:            observer.EndpointEnv{"type": "port"},
			endpointTarget: "`(`", // Invalid expression in Target (custom observer edge case)
			expectError:    "expanding discovered config",
		},
		{
			name:                     "user-specified endpoint goes into user config only",
			templateConfig:           userConfigMap{"endpoint": "`host`:`port`"},
			env:                      observer.EndpointEnv{"type": "port", "host": "192.168.1.1", "port": 8080},
			endpointTarget:           "192.168.1.1:8080",
			expectUserEndpoint:       "192.168.1.1:8080",
			expectDiscoveredEndpoint: "", // Empty - user set endpoint, nothing auto-discovered
			expectDiscoveredMarker:   false,
		},
		{
			name:                     "auto-discovered endpoint goes into discovered config",
			templateConfig:           userConfigMap{"some_field": "value"}, // No endpoint
			env:                      observer.EndpointEnv{"type": "port"},
			endpointTarget:           "192.168.1.1:8080",
			expectUserEndpoint:       "", // Empty - user didn't set endpoint
			expectDiscoveredEndpoint: "192.168.1.1:8080",
			expectDiscoveredMarker:   true, // Marker tells runner this was auto-discovered
		},
	}

	handler := &observerHandler{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := receiverTemplate{
				receiverConfig: receiverConfig{
					id:     component.MustNewID("test"),
					config: tt.templateConfig,
				},
			}
			endpoint := observer.Endpoint{ID: "test-1", Target: tt.endpointTarget}

			userConfig, discoveredConfig, err := handler.resolveConfig(template, tt.env, endpoint)

			if tt.expectError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
				return
			}

			require.NoError(t, err)

			// Check user config endpoint
			if tt.expectUserEndpoint != "" {
				assert.Equal(t, tt.expectUserEndpoint, userConfig[endpointConfigKey])
			} else {
				assert.NotContains(t, userConfig, endpointConfigKey)
			}

			// Check discovered config endpoint
			if tt.expectDiscoveredEndpoint != "" {
				assert.Equal(t, tt.expectDiscoveredEndpoint, discoveredConfig[endpointConfigKey])
			} else {
				assert.Empty(t, discoveredConfig)
			}

			// Check marker flag
			if tt.expectDiscoveredMarker {
				assert.Contains(t, discoveredConfig, tmpSetEndpointConfigKey)
			}
		})
	}
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
	factories.Receivers[component.MustNewType("filelog")] = &nopWithFilelogFactory{Factory: receivertest.NewNopFactory()}
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
