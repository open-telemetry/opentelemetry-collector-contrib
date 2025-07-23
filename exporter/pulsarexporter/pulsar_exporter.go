// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"

import (
	"context"
	"errors"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var errUnrecognizedEncoding = errors.New("unrecognized encoding")

type PulsarTracesProducer struct {
	cfg       Config
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
	if e.producer == nil {
		return nil
	}
	e.producer.Close()
	e.client.Close()
	return nil
}

func (e *PulsarTracesProducer) start(_ context.Context, _ component.Host) error {
	client, producer, err := newPulsarProducer(e.cfg)
	if err != nil {
		return err
	}
	e.client = client
	e.producer = producer
	return nil
}

type PulsarMetricsProducer struct {
	cfg       Config
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
	if e.producer == nil {
		return nil
	}
	e.producer.Close()
	e.client.Close()
	return nil
}

func (e *PulsarMetricsProducer) start(_ context.Context, _ component.Host) error {
	client, producer, err := newPulsarProducer(e.cfg)
	if err != nil {
		return err
	}
	e.client = client
	e.producer = producer
	return nil
}

type PulsarLogsProducer struct {
	cfg       Config
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
	if e.producer == nil {
		return nil
	}
	e.producer.Close()
	e.client.Close()
	return nil
}

func (e *PulsarLogsProducer) start(_ context.Context, _ component.Host) error {
	client, producer, err := newPulsarProducer(e.cfg)
	if err != nil {
		return err
	}
	e.client = client
	e.producer = producer
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

func newMetricsExporter(config Config, set exporter.Settings, marshalers map[string]MetricsMarshaler) (*PulsarMetricsProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	return &PulsarMetricsProducer{
		cfg:       config,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil
}

func newTracesExporter(config Config, set exporter.Settings, marshalers map[string]TracesMarshaler) (*PulsarTracesProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	return &PulsarTracesProducer{
		cfg:       config,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil
}

func newLogsExporter(config Config, set exporter.Settings, marshalers map[string]LogsMarshaler) (*PulsarLogsProducer, error) {
	marshaler := marshalers[config.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	return &PulsarLogsProducer{
		cfg:       config,
		topic:     config.Topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil
}
