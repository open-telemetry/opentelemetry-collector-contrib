// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"
)

func TestNewMqttExporter(t *testing.T) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Endpoint: "tcp://localhost:1883",
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "test",
					Password: "test",
				},
			},
		},
		Topic: TopicConfig{
			Topic: "test/topic",
		},
		QoS:    1,
		Retain: false,
	}

	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newMqttExporter(cfg, set.TelemetrySettings, newTestPublisherFactory(), newTestTLSFactory(), "test/topic", "test-client")

	assert.NotNil(t, exporter)
	assert.Equal(t, cfg, exporter.config)
	assert.Equal(t, "test/topic", exporter.topic)
	assert.Equal(t, "test-client", exporter.clientID)
}

func TestMqttExporterStart(t *testing.T) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Endpoint: "tcp://localhost:1883",
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "test",
					Password: "test",
				},
			},
		},
		Topic: TopicConfig{
			Topic: "test/topic",
		},
		QoS:    1,
		Retain: false,
	}

	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newMqttExporter(cfg, set.TelemetrySettings, newTestPublisherFactory(), newTestTLSFactory(), "test/topic", "test-client")

	host := componenttest.NewNopHost()
	err := exporter.start(context.Background(), host)
	require.NoError(t, err)
	assert.NotNil(t, exporter.marshaler)
}

func TestMqttExporterPublishTraces(t *testing.T) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Endpoint: "tcp://localhost:1883",
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "test",
					Password: "test",
				},
			},
		},
		Topic: TopicConfig{
			Topic: "test/topic",
		},
		QoS:    1,
		Retain: false,
	}

	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newMqttExporter(cfg, set.TelemetrySettings, newTestPublisherFactory(), newTestTLSFactory(), "test/topic", "test-client")

	host := componenttest.NewNopHost()
	err := exporter.start(context.Background(), host)
	require.NoError(t, err)

	traces := ptrace.NewTraces()
	err = exporter.publishTraces(context.Background(), traces)
	require.NoError(t, err)
}

func TestMqttExporterPublishMetrics(t *testing.T) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Endpoint: "tcp://localhost:1883",
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "test",
					Password: "test",
				},
			},
		},
		Topic: TopicConfig{
			Topic: "test/topic",
		},
		QoS:    1,
		Retain: false,
	}

	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newMqttExporter(cfg, set.TelemetrySettings, newTestPublisherFactory(), newTestTLSFactory(), "test/topic", "test-client")

	host := componenttest.NewNopHost()
	err := exporter.start(context.Background(), host)
	require.NoError(t, err)

	metrics := pmetric.NewMetrics()
	err = exporter.publishMetrics(context.Background(), metrics)
	require.NoError(t, err)
}

func TestMqttExporterPublishLogs(t *testing.T) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Endpoint: "tcp://localhost:1883",
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "test",
					Password: "test",
				},
			},
		},
		Topic: TopicConfig{
			Topic: "test/topic",
		},
		QoS:    1,
		Retain: false,
	}

	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newMqttExporter(cfg, set.TelemetrySettings, newTestPublisherFactory(), newTestTLSFactory(), "test/topic", "test-client")

	host := componenttest.NewNopHost()
	err := exporter.start(context.Background(), host)
	require.NoError(t, err)

	logs := plog.NewLogs()
	err = exporter.publishLogs(context.Background(), logs)
	require.NoError(t, err)
}

func TestMqttExporterShutdown(t *testing.T) {
	cfg := &Config{
		Connection: ConnectionConfig{
			Endpoint: "tcp://localhost:1883",
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "test",
					Password: "test",
				},
			},
		},
		Topic: TopicConfig{
			Topic: "test/topic",
		},
		QoS:    1,
		Retain: false,
	}

	set := exportertest.NewNopSettings(metadata.Type)
	exporter := newMqttExporter(cfg, set.TelemetrySettings, newTestPublisherFactory(), newTestTLSFactory(), "test/topic", "test-client")

	host := componenttest.NewNopHost()
	err := exporter.start(context.Background(), host)
	require.NoError(t, err)

	err = exporter.shutdown(context.Background())
	require.NoError(t, err)
}

func newTestPublisherFactory() publisherFactory {
	return func(dialConfig publisher.DialConfig) (publisher.Publisher, error) {
		return &testPublisher{}, nil
	}
}

func newTestTLSFactory() tlsFactory {
	return func(context.Context) (*tls.Config, error) {
		return nil, nil
	}
}

type testPublisher struct{}

func (p *testPublisher) Publish(ctx context.Context, message publisher.Message) error {
	return nil
}

func (p *testPublisher) Close() error {
	return nil
}

// Test topic templating with default values
func TestRenderWithResourceDefaultValues(t *testing.T) {
	tests := []struct {
		name     string
		template string
		attrs    map[string]string
		expected string
	}{
		{
			name:     "no template",
			template: "simple/topic",
			attrs:    map[string]string{"host.name": "test-host"},
			expected: "simple/topic",
		},
		{
			name:     "attribute found",
			template: "telemetry/data/%{resource.attributes.host.name}",
			attrs:    map[string]string{"host.name": "test-host"},
			expected: "telemetry/data/test-host",
		},
		{
			name:     "attribute not found, no default",
			template: "telemetry/data/%{resource.attributes.host.name}",
			attrs:    map[string]string{"service.name": "test-service"},
			expected: "telemetry/data/",
		},
		{
			name:     "attribute not found, with default",
			template: "telemetry/data/%{resource.attributes.host.name:unknown}",
			attrs:    map[string]string{"service.name": "test-service"},
			expected: "telemetry/data/unknown",
		},
		{
			name:     "attribute found, default ignored",
			template: "telemetry/data/%{resource.attributes.host.name:unknown}",
			attrs:    map[string]string{"host.name": "test-host"},
			expected: "telemetry/data/test-host",
		},
		{
			name:     "multiple attributes with defaults",
			template: "telemetry/data/%{resource.attributes.host.name:unknown}/%{resource.attributes.service.name:default-service}",
			attrs:    map[string]string{"host.name": "test-host"},
			expected: "telemetry/data/test-host/default-service",
		},
		{
			name:     "empty default value",
			template: "telemetry/data/%{resource.attributes.host.name:}",
			attrs:    map[string]string{"service.name": "test-service"},
			expected: "telemetry/data/",
		},
		{
			name:     "default value with special characters",
			template: "telemetry/data/%{resource.attributes.host.name:default+host#name}",
			attrs:    map[string]string{"service.name": "test-service"},
			expected: "telemetry/data/default-host-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderWithResource(tt.template, func(key string) (string, bool) {
				if val, ok := tt.attrs[key]; ok {
					return val, true
				}
				return "", false
			})
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRenderTopicFromMetricsWithDefaults(t *testing.T) {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("host.name", "test-host")
	rm.Resource().Attributes().PutStr("service.name", "test-service")

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "attribute found",
			template: "telemetry/data/%{resource.attributes.host.name}",
			expected: "telemetry/data/test-host",
		},
		{
			name:     "attribute not found, with default",
			template: "telemetry/data/%{resource.attributes.environment:production}",
			expected: "telemetry/data/production",
		},
		{
			name:     "mixed found and default",
			template: "telemetry/data/%{resource.attributes.host.name}/%{resource.attributes.environment:production}",
			expected: "telemetry/data/test-host/production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderTopicFromMetrics(tt.template, metrics)
			assert.Equal(t, tt.expected, result)
		})
	}
}
