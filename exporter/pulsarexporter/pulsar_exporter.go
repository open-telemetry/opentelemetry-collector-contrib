// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"

import (
	"context"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")

type PulsarTracesProducer struct {
	client    pulsar.Client
	producer  pulsar.Producer
	topic     string
	marshaler TracesMarshaler
	logger    *zap.Logger
}

func (e *PulsarTracesProducer) tracesPusher(ctx context.Context, td ptrace.Traces) error {
	messages, err := e.marshaler.Marshal(td, e.topic)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	var errs error
	for _, message := range messages {

		e.producer.SendAsync(ctx, message, func(_ pulsar.MessageID, _ *pulsar.ProducerMessage, err error) {
			if err != nil {
				errs = multierr.Append(errs, err)
			}
		})

	}

	return errs
}

func (e *PulsarTracesProducer) Close(context.Context) error {
	e.producer.Close()
	e.client.Close()
	return nil
}

type PulsarMetricsProducer struct {
	client    pulsar.Client
	producer  pulsar.Producer
	topic     string
	marshaler MetricsMarshaler
	logger    *zap.Logger
}

func (e *PulsarMetricsProducer) metricsDataPusher(ctx context.Context, md pmetric.Metrics) error {
	messages, err := e.marshaler.Marshal(md, e.topic)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	var errs error
	for _, message := range messages {

		e.producer.SendAsync(ctx, message, func(_ pulsar.MessageID, _ *pulsar.ProducerMessage, err error) {
			if err != nil {
				errs = multierr.Append(errs, err)
			}
		})

	}

	return errs
}

func (e *PulsarMetricsProducer) Close(context.Context) error {
	e.producer.Close()
	e.client.Close()
	return nil
}

type PulsarLogsProducer struct {
	client    pulsar.Client
	producer  pulsar.Producer
	topic     string
	marshaler LogsMarshaler
	logger    *zap.Logger
}

func (e *PulsarLogsProducer) logsDataPusher(ctx context.Context, ld plog.Logs) error {
	messages, err := e.marshaler.Marshal(ld, e.topic)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	var errs error
	for _, message := range messages {

		e.producer.SendAsync(ctx, message, func(_ pulsar.MessageID, _ *pulsar.ProducerMessage, err error) {
			if err != nil {
				errs = multierr.Append(errs, err)
			}
		})

	}

	return errs
}

func (e *PulsarLogsProducer) Close(context.Context) error {
	e.producer.Close()
	e.client.Close()
	return nil
}

func newPulsarProducer(config Config) (pulsar.Client, pulsar.Producer, error) {
	options := config.clientOptions()

	client, err := pulsar.NewClient(options)

	if err != nil {
		return nil, nil, err
	}

	producerOptions := config.getProducerOptions()

	producer, err := client.CreateProducer(producerOptions)

	if err != nil {
		return nil, nil, err
	}

	return client, producer, nil
}

func newMetricsExporter(config Config, set exporter.CreateSettings, marshalers map[string]MetricsMarshaler) (*PulsarMetricsProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	client, producer, err := newPulsarProducer(config)
	if err != nil {
		return nil, err
	}

	return &PulsarMetricsProducer{
		client:    client,
		producer:  producer,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil

}

func newTracesExporter(config Config, set exporter.CreateSettings, marshalers map[string]TracesMarshaler) (*PulsarTracesProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	client, producer, err := newPulsarProducer(config)
	if err != nil {
		return nil, err
	}
	return &PulsarTracesProducer{
		client:    client,
		producer:  producer,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil
}

func newLogsExporter(config Config, set exporter.CreateSettings, marshalers map[string]LogsMarshaler) (*PulsarLogsProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	client, producer, err := newPulsarProducer(config)
	if err != nil {
		return nil, err
	}

	return &PulsarLogsProducer{
		client:    client,
		producer:  producer,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil

}
