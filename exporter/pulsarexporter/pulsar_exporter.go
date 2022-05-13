// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarexporter

import (
	"context"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
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

	options, err := config.clientOptions()
	if err != nil {
		return nil, nil, err
	}
	client, err := pulsar.NewClient(options)

	if err != nil {
		return nil, nil, err
	}

	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Schema: pulsar.NewBytesSchema(nil),
		Topic:  config.Topic,
	})

	if err != nil {
		return nil, nil, err
	}

	return client, producer, nil
}

func newMetricsExporter(config Config, set component.ExporterCreateSettings, marshalers map[string]MetricsMarshaler) (*PulsarMetricsProducer, error) {
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

func newTracesExporter(config Config, set component.ExporterCreateSettings, marshalers map[string]TracesMarshaler) (*PulsarTracesProducer, error) {
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

func newLogsExporter(config Config, set component.ExporterCreateSettings, marshalers map[string]LogsMarshaler) (*PulsarLogsProducer, error) {
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
