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
		name           string
		templateConfig userConfigMap
		modifyEndpoint func(e observer.Endpoint) observer.Endpoint // nil means no modification
		expectRestart  bool
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

// TestResolveConfigErrors verifies that resolveConfig properly handles errors during
// template expansion. The resolveConfig method expands backtick expressions (e.g., `host`:`port`)
// in receiver configurations against endpoint environment variables. This test ensures that
// invalid expressions in both user-provided config and discovered endpoint targets produce
// appropriate errors.
func TestResolveConfigErrors(t *testing.T) {
	handler := &observerHandler{}

	t.Run("user template config expansion error", func(t *testing.T) {
		template := receiverTemplate{
			receiverConfig: receiverConfig{
				id:     component.MustNewID("test"),
				config: userConfigMap{"endpoint": "`(`"}, // Invalid expression syntax
			},
		}
		env := observer.EndpointEnv{"type": "port"}
		endpoint := observer.Endpoint{ID: "test-1", Target: "localhost:1234"}

		_, _, err := handler.resolveConfig(template, env, endpoint)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expanding user template config")
	})

	t.Run("discovered config expansion error", func(t *testing.T) {
		template := receiverTemplate{
			receiverConfig: receiverConfig{
				id:     component.MustNewID("test"),
				config: userConfigMap{}, // No endpoint set, so Target will be used
			},
		}
		env := observer.EndpointEnv{"type": "port"}
		// Simulate a custom observer that puts an invalid backtick expression in Target
		endpoint := observer.Endpoint{ID: "test-1", Target: "`(`"}

		_, _, err := handler.resolveConfig(template, env, endpoint)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expanding discovered config")
	})

	// Test the two paths for endpoint configuration:
	//
	// Path 1 (user-specified): User explicitly sets "endpoint" in their template config.
	//   - The endpoint value goes into resolvedUserConfig
	//   - resolvedDiscoveredConfig is empty (nothing to auto-discover)
	//   - Example: config: { endpoint: "`host`:`port`" }
	//
	// Path 2 (auto-discovered): User does NOT set "endpoint" in their template.
	//   - resolvedUserConfig contains only what the user specified
	//   - resolvedDiscoveredConfig contains the endpoint from e.Target, plus a marker flag
	//   - The marker (tmpSetEndpointConfigKey) tells the runner to validate if the receiver
	//     actually supports an "endpoint" field before merging
	//   - Example: config: { collection_interval: "10s" } (no endpoint)

	t.Run("user-specified endpoint goes into user config only", func(t *testing.T) {
		template := receiverTemplate{
			receiverConfig: receiverConfig{
				id:     component.MustNewID("test"),
				config: userConfigMap{"endpoint": "`host`:`port`"}, // User explicitly sets endpoint
			},
		}
		env := observer.EndpointEnv{"type": "port", "host": "192.168.1.1", "port": 8080}
		endpoint := observer.Endpoint{ID: "test-1", Target: "192.168.1.1:8080"}

		userConfig, discoveredConfig, err := handler.resolveConfig(template, env, endpoint)
		require.NoError(t, err)
		assert.Equal(t, "192.168.1.1:8080", userConfig["endpoint"])
		assert.Empty(t, discoveredConfig) // User set endpoint, so nothing auto-discovered
	})

	t.Run("auto-discovered endpoint goes into discovered config", func(t *testing.T) {
		template := receiverTemplate{
			receiverConfig: receiverConfig{
				id:     component.MustNewID("test"),
				config: userConfigMap{"some_field": "value"}, // No endpoint - will be auto-discovered
			},
		}
		env := observer.EndpointEnv{"type": "port"}
		endpoint := observer.Endpoint{ID: "test-1", Target: "192.168.1.1:8080"}

		userConfig, discoveredConfig, err := handler.resolveConfig(template, env, endpoint)
		require.NoError(t, err)

		// User config should NOT have endpoint (user didn't set it)
		assert.NotContains(t, userConfig, endpointConfigKey)
		assert.Equal(t, "value", userConfig["some_field"])

		// Discovered config should have endpoint from Target plus the marker flag
		assert.Equal(t, "192.168.1.1:8080", discoveredConfig[endpointConfigKey])
		assert.Contains(t, discoveredConfig, tmpSetEndpointConfigKey,
			"marker flag should be set so runner knows this was auto-discovered")
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
