// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqmessagereciever // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqmessagereciever"

import (
	"context"
	"crypto/tls"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqmessagereciever/internal/metadata"
)

func createDefaultConfig() component.Config {
	return &Config{
		Connection: ConnectionConfig{
			Name:     "otel-default",
			Endpoint: "127.0.0.1:5672",
			Auth: AuthConfig{
				Plain: PlainAuth{
					Username: "guest",
				},
			},
		},
	}
}

// NewFactory creates a factory for rabbitmqdata receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricReceiver, metadata.MetricsStability),
		receiver.WithLogs(createLogReceiver, metadata.MetricsStability),
		receiver.WithTraces(createTraceReceiver, metadata.MetricsStability),
	)
}

func createMetricReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg, ok := baseCfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config object")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	set := params.TelemetrySettings
	return newRabbitMQReceiver(cfg, set, nil, newTLSFactory(cfg))
}

func createLogReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Logs,
) (receiver.Logs, error) {
	cfg, ok := baseCfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config object")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	set := params.TelemetrySettings
	return newRabbitMQReceiver(cfg, set, nil, newTLSFactory(cfg))
}

func createTraceReceiver(
	_ context.Context,
	params receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Traces,
) (receiver.Traces, error) {
	cfg, ok := baseCfg.(*Config)
	if !ok {
		return nil, errors.New("invalid config object")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	set := params.TelemetrySettings
	return newRabbitMQReceiver(cfg, set, nil, newTLSFactory(cfg))
}

func newTLSFactory(config *Config) tlsFactory {
	if config.Connection.TLSConfig != nil {
		return config.Connection.TLSConfig.LoadTLSConfig
	}
	return func(context.Context) (*tls.Config, error) {
		return nil, nil
	}
}
