// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"context"
	"crypto/tls"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter/internal/publisher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/mqtt"
)

const (
	defaultConnectionTimeout          = time.Second * 10
	defaultKeepAlive                  = time.Second * 30
	defaultPublishConfirmationTimeout = time.Second * 5

	spansTopic   = "otlp/spans"
	metricsTopic = "otlp/metrics"
	logsTopic    = "otlp/logs"

	defaultSpansClientID   = "otel-collector-spans"
	defaultMetricsClientID = "otel-collector-metrics"
	defaultLogsClientID    = "otel-collector-logs"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
	)
}

func createDefaultConfig() component.Config {
	retrySettings := configretry.BackOffConfig{
		Enabled: false,
	}
	return &Config{
		QoS:           1, // At least once delivery
		Retain:        false,
		RetrySettings: retrySettings,
		Connection: ConnectionConfig{
			ConnectionTimeout:          defaultConnectionTimeout,
			KeepAlive:                  defaultKeepAlive,
			PublishConfirmationTimeout: defaultPublishConfirmationTimeout,
		},
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	config := cfg.(*Config)

	topic := getTopicOrDefault(config, spansTopic)
	clientID := defaultSpansClientID
	if config.Connection.ClientID != "" {
		clientID = config.Connection.ClientID
	}
	r := newMqttExporter(config, set.TelemetrySettings, newPublisherFactory(set), newTLSFactory(config), topic, clientID)

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		r.publishTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(r.start),
		exporterhelper.WithShutdown(r.shutdown),
		exporterhelper.WithRetry(config.RetrySettings),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	config := (cfg.(*Config))

	topic := getTopicOrDefault(config, metricsTopic)

	clientID := defaultMetricsClientID
	if config.Connection.ClientID != "" {
		clientID = config.Connection.ClientID
	}
	r := newMqttExporter(config, set.TelemetrySettings, newPublisherFactory(set), newTLSFactory(config), topic, clientID)

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		r.publishMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(r.start),
		exporterhelper.WithShutdown(r.shutdown),
		exporterhelper.WithRetry(config.RetrySettings),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	config := (cfg.(*Config))

	topic := getTopicOrDefault(config, logsTopic)
	clientID := defaultLogsClientID
	if config.Connection.ClientID != "" {
		clientID = config.Connection.ClientID
	}
	r := newMqttExporter(config, set.TelemetrySettings, newPublisherFactory(set), newTLSFactory(config), topic, clientID)

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		r.publishLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithStart(r.start),
		exporterhelper.WithShutdown(r.shutdown),
		exporterhelper.WithRetry(config.RetrySettings),
	)
}

func getTopicOrDefault(config *Config, fallback string) string {
	topic := fallback
	if config.Topic.Topic != "" {
		topic = config.Topic.Topic
	}
	return topic
}

func newPublisherFactory(set exporter.Settings) publisherFactory {
	return func(dialConfig publisher.DialConfig) (publisher.Publisher, error) {
		client := mqtt.NewMqttClient(set.Logger)
		return publisher.NewConnection(set.Logger, client, dialConfig)
	}
}

func newTLSFactory(config *Config) tlsFactory {
	if config.Connection.TLSConfig != nil {
		return config.Connection.TLSConfig.LoadTLSConfig
	}
	return func(context.Context) (*tls.Config, error) {
		return nil, nil
	}
}
