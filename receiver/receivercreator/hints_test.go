// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func TestK8sHintsBuilder(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	id := component.ID{}
	err := id.UnmarshalText([]byte("redis/pod-2-UID_6379"))
	assert.NoError(t, err)

	var config = `
collection_interval: "20s"
timeout: "30s"
username: "username"
password: "changeme"`
	var configRedis = `
collection_interval: "20s"
timeout: "130s"
username: "username"
password: "changeme"`

	tests := map[string]struct {
		inputEndpoint    observer.Endpoint
		expectedReceiver receiverTemplate
		wantError        bool
	}{
		`pod_level_hints_only`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4:6379",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery/scraper": "redis",
							"io.opentelemetry.discovery/config":  config,
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: true, traces: true},
			},
			wantError: false,
		}, `pod_level_hints_only_defaults`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4:6379",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery/scraper": "redis",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"endpoint": "1.2.3.4:6379"},
				}, signals: receiverSignals{metrics: true, logs: true, traces: true},
			},
			wantError: false,
		}, `pod_level_hints_only_signals_metrics`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4:6379",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery/scraper": "redis",
							"io.opentelemetry.discovery/signals": "metrics",
							"io.opentelemetry.discovery/config":  config,
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: false, traces: false},
			},
			wantError: false,
		},
		`pod_level_hints_only_signals_metrics_and_logs`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4:6379",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery/scraper": "redis",
							"io.opentelemetry.discovery/signals": "metrics,logs",
							"io.opentelemetry.discovery/config":  config,
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: true, traces: false},
			},
			wantError: false,
		}, `container_level_hints`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4:6379",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery.6379/scraper": "redis",
							"io.opentelemetry.discovery.6379/config":  config,
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: true, traces: true},
			},
			wantError: false,
		}, `container_level_hints_only_signals_metrics`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4:6379",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery.6379/scraper": "redis",
							"io.opentelemetry.discovery.6379/signals": "metrics",
							"io.opentelemetry.discovery/config":       config,
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: false, traces: false},
			},
			wantError: false,
		}, `mix_level_hints`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4:6379",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery.6379/scraper": "redis",
							"io.opentelemetry.discovery/config":       config,
							"io.opentelemetry.discovery.6379/config":  configRedis,
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "130s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: true, traces: true},
			},
			wantError: false,
		}, `no_port_error`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/redis(6379)",
				Target: "1.2.3.4",
				Details: &observer.Port{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							"io.opentelemetry.discovery/scraper":                    "redis",
							"io.opentelemetry.discovery/config.collection_interval": "20s",
							"io.opentelemetry.discovery/config.timeout":             "30s",
							"io.opentelemetry.discovery.6379/config.timeout":        "130s",
							"io.opentelemetry.discovery.6379/config.username":       "username",
							"io.opentelemetry.discovery.6379/config.password":       "changeme",
						}}},
			},
			expectedReceiver: receiverTemplate{},
			wantError:        true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			k8sHintsBuilder := K8sHintsBuilder{logger, K8sHintsConfig{Enabled: true}}
			env, err := test.inputEndpoint.Env()
			require.NoError(t, err)
			subreceiverTemplate, err := k8sHintsBuilder.createScraperTemplateFromHints(env)
			if !test.wantError {
				require.NoError(t, err)
				require.Equal(t, subreceiverTemplate.receiverConfig.config, test.expectedReceiver.receiverConfig.config)
				require.Equal(t, subreceiverTemplate.id, test.expectedReceiver.id)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGetConfFromAnnotations(t *testing.T) {
	var config = `
endpoint: "0.0.0.0:8080"
collection_interval: "20s"
initial_delay: "20s"
read_buffer_size: "10"
nested_example:
  foo: bar`
	var configNoEndpoint = `
collection_interval: "20s"
initial_delay: "20s"
read_buffer_size: "10"
nested_example:
  foo: bar`
	tests := map[string]struct {
		hintsAnn        map[string]string
		expectedConf    userConfigMap
		defaultEndpoint string
		scopeSuffix     string
	}{"simple_annotation_case": {
		hintsAnn: map[string]string{
			"io.opentelemetry.discovery/config": config,
		}, expectedConf: userConfigMap{
			"collection_interval": "20s",
			"endpoint":            "0.0.0.0:8080",
			"initial_delay":       "20s",
			"read_buffer_size":    "10",
			"nested_example":      userConfigMap{"foo": "bar"},
		}, defaultEndpoint: "1.2.3.4:8080",
		scopeSuffix: "",
	}, "simple_annotation_case_default_endpoint": {
		hintsAnn: map[string]string{
			"io.opentelemetry.discovery/config": configNoEndpoint,
		}, expectedConf: userConfigMap{
			"collection_interval": "20s",
			"endpoint":            "1.2.3.4:8080",
			"initial_delay":       "20s",
			"read_buffer_size":    "10",
			"nested_example":      userConfigMap{"foo": "bar"},
		}, defaultEndpoint: "1.2.3.4:8080",
		scopeSuffix: "",
	}, "simple_annotation_case_scoped": {
		hintsAnn: map[string]string{
			"io.opentelemetry.discovery.8080/config": config,
		}, expectedConf: userConfigMap{
			"collection_interval": "20s",
			"endpoint":            "0.0.0.0:8080",
			"initial_delay":       "20s",
			"read_buffer_size":    "10",
			"nested_example":      userConfigMap{"foo": "bar"},
		}, defaultEndpoint: "1.2.3.4:8080",
		scopeSuffix: "8080",
	},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(
				t,
				test.expectedConf,
				getConfFromAnnotations(test.hintsAnn, test.defaultEndpoint, test.scopeSuffix, zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))),
			)
		})
	}
}

func TestDiscoveryEnabled(t *testing.T) {
	var config = `
endpoint: "0.0.0.0:8080"`
	tests := map[string]struct {
		hintsAnn    map[string]string
		expected    bool
		scopeSuffix string
	}{
		"test_enabled": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery/config":  config,
				"io.opentelemetry.discovery/enabled": "true",
			},
			expected:    true,
			scopeSuffix: "",
		}, "test_disabled": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery/config":  config,
				"io.opentelemetry.discovery/enabled": "false",
			},
			expected:    false,
			scopeSuffix: "",
		}, "test_default": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery/config": config,
			},
			expected:    true,
			scopeSuffix: "",
		}, "test_enabled_scope": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery/config":       config,
				"io.opentelemetry.discovery.8080/enabled": "true",
			},
			expected:    true,
			scopeSuffix: "some",
		}, "test_disabled_scoped": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery/config":       config,
				"io.opentelemetry.discovery.8080/enabled": "false",
			},
			expected:    false,
			scopeSuffix: "8080",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(
				t,
				test.expected,
				discoveryEnabled(test.hintsAnn, test.scopeSuffix),
			)
		})
	}
}
