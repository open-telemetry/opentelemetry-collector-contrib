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

	id := component.ID{}
	err := id.UnmarshalText([]byte("redis/pod-2-UID_6379"))
	assert.NoError(t, err)

	config := `
collection_interval: "20s"
timeout: "30s"
username: "username"
password: "changeme"`
	configRedis := `
collection_interval: "20s"
timeout: "130s"
username: "username"
password: "changeme"`

	tests := map[string]struct {
		inputEndpoint    observer.Endpoint
		expectedReceiver receiverTemplate
		ignoreReceivers  []string
		wantError        bool
	}{
		`metrics_pod_level_hints_only`: {
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
							otelMetricsHints + "/enabled": "true",
							otelMetricsHints + "/scraper": "redis",
							otelMetricsHints + "/config":  config,
						},
					},
					Port: 6379,
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: false, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `metrics_pod_level_ignore`: {
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
							otelMetricsHints + "/enabled": "true",
							otelMetricsHints + "/scraper": "redis",
							otelMetricsHints + "/config":  config,
						},
					},
					Port: 6379,
				},
			},
			expectedReceiver: receiverTemplate{},
			wantError:        false,
			ignoreReceivers:  []string{"redis"},
		}, `metrics_pod_level_hints_only_defaults`: {
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
							otelMetricsHints + "/enabled": "true",
							otelMetricsHints + "/scraper": "redis",
						},
					},
					Port: 6379,
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"endpoint": "1.2.3.4:6379"},
				}, signals: receiverSignals{metrics: true, logs: false, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `metrics_container_level_hints`: {
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
							otelMetricsHints + ".6379/enabled": "true",
							otelMetricsHints + ".6379/scraper": "redis",
							otelMetricsHints + ".6379/config":  config,
						},
					},
					Port: 6379,
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: false, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `metrics_mix_level_hints`: {
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
							otelMetricsHints + ".6379/enabled": "true",
							otelMetricsHints + ".6379/scraper": "redis",
							otelMetricsHints + "/config":       config,
							otelMetricsHints + ".6379/config":  configRedis,
						},
					},
					Port: 6379,
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "130s", "username": "username"},
				}, signals: receiverSignals{metrics: true, logs: false, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `metrics_no_port_error`: {
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
							otelMetricsHints + "/enabled": "true",
							otelMetricsHints + "/scraper": "redis",
							otelMetricsHints + "/config":  config,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{},
			wantError:        true,
			ignoreReceivers:  []string{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			builder := createK8sHintsBuilder(DiscoveryConfig{Enabled: true, IgnoreReceivers: test.ignoreReceivers}, logger)
			env, err := test.inputEndpoint.Env()
			require.NoError(t, err)
			subreceiverTemplate, err := builder.createReceiverTemplateFromHints(env)
			if subreceiverTemplate == nil {
				require.Equal(t, receiverTemplate{}, test.expectedReceiver)
				return
			}
			if !test.wantError {
				require.NoError(t, err)
				require.Equal(t, subreceiverTemplate.config, test.expectedReceiver.config)
				require.Equal(t, subreceiverTemplate.signals, test.expectedReceiver.signals)
				require.Equal(t, subreceiverTemplate.id, test.expectedReceiver.id)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestK8sHintsBuilderLogs(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))

	id := component.ID{}
	err := id.UnmarshalText([]byte("filelog/pod-2-UID_redis"))
	assert.NoError(t, err)

	idNginx := component.ID{}
	err = idNginx.UnmarshalText([]byte("filelog/pod-2-UID_nginx"))
	assert.NoError(t, err)

	config := `
include_file_name: true
max_log_size: "2MiB"
operators:
- type: container
  id: container-parser
- type: regex_parser
  regex: "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"`
	configNginx := `
include_file_name: true
max_log_size: "4MiB"
operators:
- type: container
  id: container-parser
- type: add
  field: attributes.tag
  value: beta`
	configReplaceContainer := `
operators:
- type: container
  id: replaced
- type: regex_parser
  regex: "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"`
	configReplaceInclude := `
include:
- /var/log/pod/x/foo.log`

	tests := map[string]struct {
		inputEndpoint      observer.Endpoint
		expectedReceiver   receiverTemplate
		wantError          bool
		ignoreReceivers    []string
		defaultAnnotations map[string]string
	}{
		`logs_pod_level_hints_only`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + "/enabled": "true",
							otelLogsHints + "/config":  config,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id: id,
					config: userConfigMap{
						"include":           []string{"/var/log/pods/default_pod-2_pod-2-UID/redis/*.log"},
						"include_file_name": true,
						"include_file_path": true,
						"max_log_size":      "2MiB",
						"operators": []any{
							map[string]any{"id": "container-parser", "type": "container"},
							map[string]any{"type": "regex_parser", "regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"},
						},
					},
				}, signals: receiverSignals{metrics: false, logs: true, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `logs_pod_level_hints_only_ignore_receiver`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + "/enabled": "true",
							otelLogsHints + "/config":  config,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{},
			wantError:        false,
			ignoreReceivers:  []string{logsReceiver},
		}, `logs_pod_level_hints_only_defaults`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:6379",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + "/enabled": "true",
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id: id,
					config: userConfigMap{
						"include":           []string{"/var/log/pods/default_pod-2_pod-2-UID/redis/*.log"},
						"include_file_name": false,
						"include_file_path": true,
						"operators": []any{
							map[string]any{"id": "container-parser", "type": "container"},
						},
					},
				}, signals: receiverSignals{metrics: false, logs: true, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `logs_pod_level_hints_default_all`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:6379",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:        "pod-2",
						Namespace:   "default",
						UID:         "pod-2-UID",
						Labels:      map[string]string{"env": "prod"},
						Annotations: map[string]string{},
					},
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id: id,
					config: userConfigMap{
						"include":           []string{"/var/log/pods/default_pod-2_pod-2-UID/redis/*.log"},
						"include_file_name": false,
						"include_file_path": true,
						"operators": []any{
							map[string]any{"id": "container-parser", "type": "container"},
						},
					},
				}, signals: receiverSignals{metrics: false, logs: true, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
			defaultAnnotations: map[string]string{
				otelLogsHints + "/enabled": "true",
			},
		}, `logs_pod_level_hints_disable_default_all`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:6379",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + "/enabled": "false",
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{},
			wantError:        false,
			ignoreReceivers:  []string{},
			defaultAnnotations: map[string]string{
				otelLogsHints + "/enabled": "true",
			},
		}, `logs_container_level_hints`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:6379",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + ".redis/enabled": "true",
							otelLogsHints + ".redis/config":  config,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id: id,
					config: userConfigMap{
						"include":           []string{"/var/log/pods/default_pod-2_pod-2-UID/redis/*.log"},
						"include_file_name": true,
						"include_file_path": true,
						"max_log_size":      "2MiB",
						"operators": []any{
							map[string]any{"id": "container-parser", "type": "container"},
							map[string]any{"type": "regex_parser", "regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"},
						},
					},
				}, signals: receiverSignals{metrics: false, logs: true, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `logs_mix_level_hints`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:6379",
				Details: &observer.PodContainer{
					Name: "nginx", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + ".nginx/enabled": "true",
							otelLogsHints + "/config":        config,
							otelLogsHints + ".nginx/config":  configNginx,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id: idNginx,
					config: userConfigMap{
						"include":           []string{"/var/log/pods/default_pod-2_pod-2-UID/nginx/*.log"},
						"include_file_name": true,
						"include_file_path": true,
						"max_log_size":      "4MiB",
						"operators": []any{
							map[string]any{"id": "container-parser", "type": "container"},
							map[string]any{"type": "add", "field": "attributes.tag", "value": "beta"},
						},
					},
				}, signals: receiverSignals{metrics: false, logs: true, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `logs_no_container_error`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4",
				Details: &observer.PodContainer{
					Name: "", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + ".nginx/enabled": "true",
							otelLogsHints + "/config":        config,
							otelLogsHints + ".nginx/config":  configNginx,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{},
			wantError:        true,
			ignoreReceivers:  []string{},
		}, `logs_pod_level_hints_replace_container_operator`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + "/enabled": "true",
							otelLogsHints + "/config":  configReplaceContainer,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id: id,
					config: userConfigMap{
						"include":           []string{"/var/log/pods/default_pod-2_pod-2-UID/redis/*.log"},
						"include_file_name": false,
						"include_file_path": true,
						"operators": []any{
							map[string]any{"id": "replaced", "type": "container"},
							map[string]any{"type": "regex_parser", "regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"},
						},
					},
				}, signals: receiverSignals{metrics: false, logs: true, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		}, `logs_pod_level_hints_do_not_replace_path`: {
			inputEndpoint: observer.Endpoint{
				ID:     "namespace/pod-2-UID/filelog(redis)",
				Target: "1.2.3.4:",
				Details: &observer.PodContainer{
					Name: "redis", Pod: observer.Pod{
						Name:      "pod-2",
						Namespace: "default",
						UID:       "pod-2-UID",
						Labels:    map[string]string{"env": "prod"},
						Annotations: map[string]string{
							otelLogsHints + "/enabled": "true",
							otelLogsHints + "/config":  configReplaceInclude,
						},
					},
				},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id: id,
					config: userConfigMap{
						"include":           []string{"/var/log/pods/default_pod-2_pod-2-UID/redis/*.log"},
						"include_file_name": false,
						"include_file_path": true,
						"operators": []any{
							map[string]any{"id": "container-parser", "type": "container"},
						},
					},
				}, signals: receiverSignals{metrics: false, logs: true, traces: false},
			},
			wantError:       false,
			ignoreReceivers: []string{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			builder := createK8sHintsBuilder(
				DiscoveryConfig{
					Enabled:            true,
					IgnoreReceivers:    test.ignoreReceivers,
					DefaultAnnotations: test.defaultAnnotations,
				},
				logger)
			env, err := test.inputEndpoint.Env()
			require.NoError(t, err)
			subreceiverTemplate, err := builder.createReceiverTemplateFromHints(env)
			if subreceiverTemplate == nil {
				require.Equal(t, receiverTemplate{}, test.expectedReceiver)
				return
			}
			if !test.wantError {
				require.NoError(t, err)
				require.Equal(t, subreceiverTemplate.config, test.expectedReceiver.config)
				require.Equal(t, subreceiverTemplate.signals, test.expectedReceiver.signals)
				require.Equal(t, subreceiverTemplate.id, test.expectedReceiver.id)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestGetConfFromAnnotations(t *testing.T) {
	config := `
endpoint: "0.0.0.0:8080"
collection_interval: "20s"
initial_delay: "20s"
read_buffer_size: "10"
nested_example:
  foo: bar`
	configNoEndpoint := `
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
		expectError     bool
	}{
		"simple_annotation_case": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics/enabled": "true",
				"io.opentelemetry.discovery.metrics/config":  config,
			}, expectedConf: userConfigMap{
				"collection_interval": "20s",
				"endpoint":            "0.0.0.0:8080",
				"initial_delay":       "20s",
				"read_buffer_size":    "10",
				"nested_example":      userConfigMap{"foo": "bar"},
			}, defaultEndpoint: "0.0.0.0:8080",
			scopeSuffix: "",
		}, "simple_annotation_case_default_endpoint": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics/enabled": "true",
				"io.opentelemetry.discovery.metrics/config":  configNoEndpoint,
			}, expectedConf: userConfigMap{
				"collection_interval": "20s",
				"endpoint":            "1.1.1.1:8080",
				"initial_delay":       "20s",
				"read_buffer_size":    "10",
				"nested_example":      userConfigMap{"foo": "bar"},
			}, defaultEndpoint: "1.1.1.1:8080",
			scopeSuffix: "",
		}, "simple_annotation_case_scoped": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics.8080/enabled": "true",
				"io.opentelemetry.discovery.metrics.8080/config":  config,
			}, expectedConf: userConfigMap{
				"collection_interval": "20s",
				"endpoint":            "0.0.0.0:8080",
				"initial_delay":       "20s",
				"read_buffer_size":    "10",
				"nested_example":      userConfigMap{"foo": "bar"},
			}, defaultEndpoint: "0.0.0.0:8080",
			scopeSuffix: "8080",
		}, "simple_annotation_case_with_invalid_endpoint": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics/enabled": "true",
				"io.opentelemetry.discovery.metrics/config":  config,
			}, expectedConf: userConfigMap{},
			defaultEndpoint: "1.2.3.4:8080",
			scopeSuffix:     "",
			expectError:     true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			conf, err := getScraperConfFromAnnotations(test.hintsAnn, test.defaultEndpoint, test.scopeSuffix, zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel)))
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(
					t,
					test.expectedConf,
					conf)
			}
		})
	}
}

func TestCreateLogsConfig(t *testing.T) {
	config := `
include_file_name: true
max_log_size: "2MiB"
operators:
- type: container
  id: container-parser
- type: regex_parser
  regex: "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"`
	configRegex := `
include_file_name: true
max_log_size: "2MiB"
operators:
- type: container
  id: container-parser
- type: regex_parser
  regex: ^(?P<source_ip>\d+\.\d+.\d+\.\d+)\s+-\s+-\s+\[(?P<timestamp_log>\d+/\w+/\d+:\d+:\d+:\d+\s+\+\d+)\]\s"(?P<http_method>\w+)\s+(?P<http_path>.*)\s+(?P<http_version>.*)"\s+(?P<http_code>\d+)\s+(?P<http_size>\d+)$`
	tests := map[string]struct {
		hintsAnn        map[string]string
		expectedConf    userConfigMap
		defaultEndpoint string
	}{
		"simple_annotation_case": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.logs/config": config,
			}, expectedConf: userConfigMap{
				"include":           []string{"/var/log/pods/my-ns_my-pod_my-uid/my-container/*.log"},
				"include_file_name": true,
				"include_file_path": true,
				"max_log_size":      "2MiB",
				"operators": []any{
					map[string]any{"id": "container-parser", "type": "container"},
					map[string]any{"type": "regex_parser", "regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$"},
				},
			}, defaultEndpoint: "1.2.3.4:8080",
		}, "config_annotation_case": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.logs/config": configRegex,
			}, expectedConf: userConfigMap{
				"include":           []string{"/var/log/pods/my-ns_my-pod_my-uid/my-container/*.log"},
				"include_file_name": true,
				"include_file_path": true,
				"max_log_size":      "2MiB",
				"operators": []any{
					map[string]any{"id": "container-parser", "type": "container"},
					map[string]any{"type": "regex_parser", "regex": `^(?P<source_ip>\d+\.\d+.\d+\.\d+)\s+-\s+-\s+\[(?P<timestamp_log>\d+/\w+/\d+:\d+:\d+:\d+\s+\+\d+)\]\s"(?P<http_method>\w+)\s+(?P<http_path>.*)\s+(?P<http_version>.*)"\s+(?P<http_code>\d+)\s+(?P<http_size>\d+)$`},
				},
			}, defaultEndpoint: "1.2.3.4:8080",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(
				t,
				test.expectedConf,
				createLogsConfig(
					test.hintsAnn,
					"my-container",
					"my-uid",
					"my-pod",
					"my-ns",
					zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))),
			)
		})
	}
}

func TestDiscoveryEnabled(t *testing.T) {
	config := `
endpoint: "0.0.0.0:8080"`
	tests := map[string]struct {
		hintsAnn    map[string]string
		expected    bool
		scopeSuffix string
	}{
		"test_enabled": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics/config":  config,
				"io.opentelemetry.discovery.metrics/enabled": "true",
			},
			expected:    true,
			scopeSuffix: "",
		}, "test_disabled": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics/config":  config,
				"io.opentelemetry.discovery.metrics/enabled": "false",
			},
			expected:    false,
			scopeSuffix: "",
		}, "test_enabled_scope": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics/config":       config,
				"io.opentelemetry.discovery.metrics.8080/enabled": "true",
			},
			expected:    true,
			scopeSuffix: "8080",
		}, "test_disabled_scoped": {
			hintsAnn: map[string]string{
				"io.opentelemetry.discovery.metrics/config":       config,
				"io.opentelemetry.discovery.metrics.8080/enabled": "false",
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
				discoveryEnabled(test.hintsAnn, otelMetricsHints, test.scopeSuffix),
			)
		})
	}
}

func TestValidateEndpoint(t *testing.T) {
	tests := map[string]struct {
		endpoint        string
		defaultEndpoint string
		expectError     bool
	}{
		"test_valid": {
			endpoint:        "http://1.2.3.4:8080/stats",
			defaultEndpoint: "1.2.3.4:8080",
			expectError:     false,
		},
		"test_invalid": {
			endpoint:        "http://0.0.0.0:8080/some?foo=1.2.3.4:8080",
			defaultEndpoint: "1.2.3.4:8080",
			expectError:     true,
		},
		"test_valid_no_scheme": {
			endpoint:        "1.2.3.4:8080/stats",
			defaultEndpoint: "1.2.3.4:8080",
			expectError:     false,
		},
		"test_valid_no_scheme_no_path": {
			endpoint:        "1.2.3.4:8080",
			defaultEndpoint: "1.2.3.4:8080",
			expectError:     false,
		},
		"test_valid_no_scheme_dynamic": {
			endpoint:        "`endpoint`/stats",
			defaultEndpoint: "1.2.3.4:8080",
			expectError:     false,
		},
		"test_valid_dynamic": {
			endpoint:        "http://`endpoint`/stats",
			defaultEndpoint: "1.2.3.4:8080",
			expectError:     false,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			err := validateEndpoint(test.endpoint, test.defaultEndpoint)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
