// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter"

import (
	"context"
	"crypto/tls"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter/internal/publisher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/rabbitmq"
)

type rabbitmqExporter struct {
	config *Config
	tlsFactory
	settings       component.TelemetrySettings
	routingKey     string
	connectionName string
	*marshaler
	publisherFactory
	publisher publisher.Publisher
}

type (
	publisherFactory = func(publisher.DialConfig) (publisher.Publisher, error)
	tlsFactory       = func(context.Context) (*tls.Config, error)
)

func newRabbitmqExporter(cfg *Config, set component.TelemetrySettings, publisherFactory publisherFactory, tlsFactory tlsFactory, routingKey string, connectionName string) *rabbitmqExporter {
	exporter := &rabbitmqExporter{
		config:           cfg,
		settings:         set,
		routingKey:       routingKey,
		connectionName:   connectionName,
		publisherFactory: publisherFactory,
		tlsFactory:       tlsFactory,
	}
	return exporter
}

func (e *rabbitmqExporter) start(ctx context.Context, host component.Host) error {
	m, err := newMarshaler(e.config.EncodingExtensionID, host)
	if err != nil {
		return err
	}
	e.marshaler = m

	dialConfig := publisher.DialConfig{
		Durable:                    e.config.Durable,
		PublishConfirmationTimeout: e.config.Connection.PublishConfirmationTimeout,
		DialConfig: rabbitmq.DialConfig{
			URL:   e.config.Connection.Endpoint,
			Vhost: e.config.Connection.VHost,
			Auth: &amqp.PlainAuth{
				Username: e.config.Connection.Auth.Plain.Username,
				Password: e.config.Connection.Auth.Plain.Password,
			},
			ConnectionName:    e.connectionName,
			ConnectionTimeout: e.config.Connection.ConnectionTimeout,
			Heartbeat:         e.config.Connection.Heartbeat,
		},
	}

	tlsConfig, err := e.tlsFactory(ctx)
	if err != nil {
		return err
	}
	dialConfig.TLS = tlsConfig

	e.settings.Logger.Info("Establishing initial connection to RabbitMQ")
	p, err := e.publisherFactory(dialConfig)
	e.publisher = p

	if err != nil {
		return err
	}

	return nil
}

func (e *rabbitmqExporter) publishTraces(context context.Context, traces ptrace.Traces) error {
	body, err := e.tracesMarshaler.MarshalTraces(traces)
	if err != nil {
		return err
	}

	message := publisher.Message{
		Exchange:   e.config.Routing.Exchange,
		RoutingKey: e.routingKey,
		Body:       body,
	}
	return e.publisher.Publish(context, message)
}

func (e *rabbitmqExporter) publishMetrics(context context.Context, metrics pmetric.Metrics) error {
	body, err := e.metricsMarshaler.MarshalMetrics(metrics)
	if err != nil {
		return err
	}

	message := publisher.Message{
		Exchange:   e.config.Routing.Exchange,
		RoutingKey: e.routingKey,
		Body:       body,
	}
	return e.publisher.Publish(context, message)
}

func (e *rabbitmqExporter) publishLogs(context context.Context, logs plog.Logs) error {
	body, err := e.logsMarshaler.MarshalLogs(logs)
	if err != nil {
		return err
	}

	message := publisher.Message{
		Exchange:   e.config.Routing.Exchange,
		RoutingKey: e.routingKey,
		Body:       body,
	}
	return e.publisher.Publish(context, message)
}

func (e *rabbitmqExporter) shutdown(_ context.Context) error {
	if e.publisher != nil {
		return e.publisher.Close()
	}
	return nil
}
