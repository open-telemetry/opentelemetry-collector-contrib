// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"

import (
	"context"
	"crypto/tls"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/publisher"
)

const (
	defaultConnectionTimeout          = time.Second * 10
	defaultConnectionHeartbeat        = time.Second * 5
	defaultPublishConfirmationTimeout = time.Second * 5

	spansRoutingKey   = "otlp_spans"
	metricsRoutingKey = "otlp_metrics"
	logsRoutingKey    = "otlp_logs"

	spansConnectionName   = "otel-collector-spans"
	metricsConnectionName = "otel-collector-metrics"
	logsConnectionName    = "otel-collector-logs"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.TracesStability),
		exporter.WithTraces(createTracesExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	retrySettings := configretry.BackOffConfig{
		Enabled: false,
	}
	return &Config{
		Durable:       true,
		RetrySettings: retrySettings,
		Connection: ConnectionConfig{
			ConnectionTimeout:          defaultConnectionTimeout,
			Heartbeat:                  defaultConnectionHeartbeat,
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

	routingKey := getRoutingKeyOrDefault(config, spansRoutingKey)
	r := newRabbitmqExporter(config, set.TelemetrySettings, newPublisherFactory(set), newTLSFactory(config), routingKey, spansConnectionName)

	return exporterhelper.NewTracesExporter(
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

	routingKey := getRoutingKeyOrDefault(config, metricsRoutingKey)
	r := newRabbitmqExporter(config, set.TelemetrySettings, newPublisherFactory(set), newTLSFactory(config), routingKey, metricsConnectionName)

	return exporterhelper.NewMetricsExporter(
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

	routingKey := getRoutingKeyOrDefault(config, logsRoutingKey)
	r := newRabbitmqExporter(config, set.TelemetrySettings, newPublisherFactory(set), newTLSFactory(config), routingKey, logsConnectionName)

	return exporterhelper.NewLogsExporter(
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

func getRoutingKeyOrDefault(config *Config, fallback string) string {
	routingKey := fallback
	if config.Routing.RoutingKey != "" {
		routingKey = config.Routing.RoutingKey
	}
	return routingKey
}

func newPublisherFactory(set exporter.Settings) publisherFactory {
	return func(dialConfig publisher.DialConfig) (publisher.Publisher, error) {
		return publisher.NewConnection(set.Logger, publisher.NewAmqpClient(), dialConfig)
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
