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
							"io.opentelemetry.collector.receiver-creator.metrics/receiver":            "redis",
							"io.opentelemetry.collector.receiver-creator.metrics/collection_interval": "20s",
							"io.opentelemetry.collector.receiver-creator.metrics/timeout":             "30s",
							"io.opentelemetry.collector.receiver-creator.metrics/username":            "username",
							"io.opentelemetry.collector.receiver-creator.metrics/password":            "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				},
			},
			wantError: false,
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
							"io.opentelemetry.collector.receiver-creator.metrics.redis/receiver":            "redis",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/collection_interval": "20s",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/timeout":             "30s",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/username":            "username",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/password":            "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "30s", "username": "username"},
				},
			},
			wantError: false,
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
							"io.opentelemetry.collector.receiver-creator.metrics.redis/receiver":      "redis",
							"io.opentelemetry.collector.receiver-creator.metrics/collection_interval": "20s",
							"io.opentelemetry.collector.receiver-creator.metrics/timeout":             "30s",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/timeout":       "130s",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/username":      "username",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/password":      "changeme",
						}},
					Port: 6379},
			},
			expectedReceiver: receiverTemplate{
				receiverConfig: receiverConfig{
					id:     id,
					config: userConfigMap{"collection_interval": "20s", "endpoint": "1.2.3.4:6379", "password": "changeme", "timeout": "130s", "username": "username"},
				},
			},
			wantError: false,
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
							"io.opentelemetry.collector.receiver-creator.metrics.redis/receiver":      "redis",
							"io.opentelemetry.collector.receiver-creator.metrics/collection_interval": "20s",
							"io.opentelemetry.collector.receiver-creator.metrics/timeout":             "30s",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/timeout":       "130s",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/username":      "username",
							"io.opentelemetry.collector.receiver-creator.metrics.redis/password":      "changeme",
						}}},
			},
			expectedReceiver: receiverTemplate{},
			wantError:        true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			k8sHintsBuilder := K8sHintsBuilder{logger, K8sHintsConfig{Metrics: MetricsHints{Enabled: true}}}
			env, err := test.inputEndpoint.Env()
			require.NoError(t, err)
			subreceiverTemplate, err := k8sHintsBuilder.createReceiverTemplateFromHints(env)
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

func TestGetHintAnnotation(t *testing.T) {
	metricsHintsAnn := map[string]string{
		"io.opentelemetry.collector.receiver-creator.metrics/receiver": "redis",
	}
	assert.Equal(
		t,
		"redis",
		getHintAnnotation(metricsHintsAnn, metricsHint, hintsMetricsReceiver, "webport"),
	)
	logsHintsAnn := map[string]string{
		"io.opentelemetry.collector.receiver-creator.logs/receiver": "redis",
	}
	assert.Equal(
		t,
		"redis",
		getHintAnnotation(logsHintsAnn, "logs", "receiver", "webport"),
	)
}
