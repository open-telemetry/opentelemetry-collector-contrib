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

func TestK8sHintsBuilderMetrics(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))
	logger.Level()

	id := component.ID{}
	err := id.UnmarshalText([]byte("redis/pod-2-UID_6379"))
	assert.NoError(t, err)

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
							"opentelemetry.io.discovery/scraper":                    "redis",
							"opentelemetry.io.discovery/config.collection_interval": "20s",
							"opentelemetry.io.discovery/config.timeout":             "30s",
							"opentelemetry.io.discovery/config.username":            "username",
							"opentelemetry.io.discovery/config.password":            "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username", "signals": receiverSignals{metrics: true, logs: true, traces: true}},
				},
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
							"opentelemetry.io.discovery/scraper":                    "redis",
							"opentelemetry.io.discovery/signals":                    "metrics",
							"opentelemetry.io.discovery/config.collection_interval": "20s",
							"opentelemetry.io.discovery/config.timeout":             "30s",
							"opentelemetry.io.discovery/config.username":            "username",
							"opentelemetry.io.discovery/config.password":            "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username", "signals": receiverSignals{metrics: true, logs: false, traces: false}},
				},
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
							"opentelemetry.io.discovery/scraper":                    "redis",
							"opentelemetry.io.discovery/signals":                    "metrics,logs",
							"opentelemetry.io.discovery/config.collection_interval": "20s",
							"opentelemetry.io.discovery/config.timeout":             "30s",
							"opentelemetry.io.discovery/config.username":            "username",
							"opentelemetry.io.discovery/config.password":            "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username", "signals": receiverSignals{metrics: true, logs: true, traces: false}},
				},
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
							"opentelemetry.io.discovery.redis/scraper":                    "redis",
							"opentelemetry.io.discovery.redis/config.collection_interval": "20s",
							"opentelemetry.io.discovery.redis/config.timeout":             "30s",
							"opentelemetry.io.discovery.redis/config.username":            "username",
							"opentelemetry.io.discovery.redis/config.password":            "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username", "signals": receiverSignals{metrics: true, logs: true, traces: true}},
				},
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
							"opentelemetry.io.discovery.redis/scraper":              "redis",
							"opentelemetry.io.discovery.redis/signals":              "metrics",
							"opentelemetry.io.discovery/config.collection_interval": "20s",
							"opentelemetry.io.discovery/config.timeout":             "30s",
							"opentelemetry.io.discovery/config.username":            "username",
							"opentelemetry.io.discovery/config.password":            "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username", "signals": receiverSignals{metrics: true, logs: false, traces: false}},
				},
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
							"opentelemetry.io.discovery.redis/scraper":              "redis",
							"opentelemetry.io.discovery/config.collection_interval": "20s",
							"opentelemetry.io.discovery/config.timeout":             "30s",
							"opentelemetry.io.discovery.redis/config.timeout":       "130s",
							"opentelemetry.io.discovery.redis/config.username":      "username",
							"opentelemetry.io.discovery.redis/config.password":      "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "130s", "username": "username", "signals": receiverSignals{metrics: true, logs: true, traces: true}},
				},
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
							"opentelemetry.io.discovery/scraper":                    "redis",
							"opentelemetry.io.discovery/config.collection_interval": "20s",
							"opentelemetry.io.discovery/config.timeout":             "30s",
							"opentelemetry.io.discovery.redis/config.timeout":       "130s",
							"opentelemetry.io.discovery.redis/config.username":      "username",
							"opentelemetry.io.discovery.redis/config.password":      "changeme",
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
	var nestedMap = `
entries: 
  - keya1: val1
    keya2: val2
foo: bar`
	var nestedList = `
- type: regex_parser
  regex: '^(?P<time>\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$'
- type: add
  key: body.some
  value: awesome`

	tests := map[string]struct {
		hintsAnn        map[string]string
		expectedConf    userConfigMap
		defaultEndpoint string
		scopeSuffix     string
	}{"simple_annotation_case": {
		hintsAnn: map[string]string{
			"opentelemetry.io.discovery/config.endpoint":            "0.0.0.0:8080",
			"opentelemetry.io.discovery/config.collection_interval": "20s",
			"opentelemetry.io.discovery/config.initial_delay":       "20s",
			"opentelemetry.io.discovery/config.read_buffer_size":    "10",
		}, expectedConf: userConfigMap{
			"collection_interval": "20s",
			"endpoint":            "0.0.0.0:8080",
			"initial_delay":       "20s",
			"read_buffer_size":    "10",
		}, defaultEndpoint: "",
		scopeSuffix: "",
	}, "nested_annotation_case": {
		hintsAnn: map[string]string{
			"opentelemetry.io.discovery/config.read_buffer_size":        "10",
			"opentelemetry.io.discovery/config.read_buffer_size_nested": nestedMap,
		}, expectedConf: userConfigMap{
			"read_buffer_size": "10",
			"read_buffer_size_nested": map[string]any{
				"foo": "bar",
				"entries": []any{map[string]any{
					"keya1": "val1",
					"keya2": "val2"}},
			},
		}, defaultEndpoint: "",
		scopeSuffix: "",
	}, "nested_list_annotation_case": {
		hintsAnn: map[string]string{
			"opentelemetry.io.discovery.webport/config.read_buffer_size": "10",
			"opentelemetry.io.discovery/config.read_buffer_size":         "20",
			"opentelemetry.io.discovery/config.listconf":                 nestedList,
		}, expectedConf: userConfigMap{
			"read_buffer_size": "10",
			"listconf": []any{map[string]any{
				"type":  "regex_parser",
				"regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"},
				map[string]any{
					"type":  "add",
					"key":   "body.some",
					"value": "awesome"}},
		}, defaultEndpoint: "",
		scopeSuffix: "webport",
	},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(
				t,
				test.expectedConf,
				getConfFromAnnotations(test.hintsAnn, test.defaultEndpoint, test.scopeSuffix),
			)
		})
	}
}

func TestDiscoveryEnabled(t *testing.T) {
	tests := map[string]struct {
		hintsAnn    map[string]string
		expected    bool
		scopeSuffix string
	}{
		"test_enabled": {
			hintsAnn: map[string]string{
				"opentelemetry.io.discovery/config.endpoint": "0.0.0.0:8080",
				"opentelemetry.io.discovery/enabled":         "true",
			},
			expected:    true,
			scopeSuffix: "",
		}, "test_disabled": {
			hintsAnn: map[string]string{
				"opentelemetry.io.discovery/config.endpoint": "0.0.0.0:8080",
				"opentelemetry.io.discovery/enabled":         "false",
			},
			expected:    false,
			scopeSuffix: "",
		}, "test_default": {
			hintsAnn: map[string]string{
				"opentelemetry.io.discovery/config.endpoint": "0.0.0.0:8080",
			},
			expected:    true,
			scopeSuffix: "",
		}, "test_enabled_scope": {
			hintsAnn: map[string]string{
				"opentelemetry.io.discovery/config.endpoint": "0.0.0.0:8080",
				"opentelemetry.io.discovery.some/enabled":    "true",
			},
			expected:    true,
			scopeSuffix: "some",
		}, "test_disabled_scoped": {
			hintsAnn: map[string]string{
				"opentelemetry.io.discovery/config.endpoint": "0.0.0.0:8080",
				"opentelemetry.io.discovery.some/enabled":    "false",
			},
			expected:    false,
			scopeSuffix: "some",
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
