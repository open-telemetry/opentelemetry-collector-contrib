// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
import (
	"context"
	"errors"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

var errMissMetricsTopic = errors.New("didn't specify topic for metrics exporting, skip start metrics exporter")

type pulsarMetricsProducer struct {
	config    Config
	client    pulsar.Client
	producer  pulsar.Producer
	topic     string
	marshaler MetricsMarshaler
	logger    *zap.Logger
}

func (e *pulsarMetricsProducer) metricsDataPusher(ctx context.Context, md pmetric.Metrics) error {
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

// Start component.StartFunc
func (e *pulsarMetricsProducer) Start(context.Context, component.Host) error {
	client, producer, err := e.config.createPulsarProducer(e.topic)
	e.producer = producer
	e.client = client
	return err
}

// Close component.Shutdown
func (e *pulsarMetricsProducer) Close(context.Context) error {
	if e.producer != nil {
		e.producer.Close()
	}
	if e.client != nil {
		e.client.Close()
	}
	return nil
}

func newMetricsExporter(config Config, set exporter.CreateSettings, marshalers map[string]MetricsMarshaler) (*pulsarMetricsProducer, error) {
	option := config.Metric
	if len(option.Encoding) == 0 {
		option.Encoding = defaultEncoding
	}
	marshaler := marshalers[option.Encoding]
	if marshaler == nil {
		return nil, errUnrecognizedEncoding
	}
	topic := option.Topic
	if len(topic) == 0 {
		return nil, errMissMetricsTopic
	}
	return &pulsarMetricsProducer{
		config:    config,
		topic:     topic,
		marshaler: marshaler,
		logger:    set.Logger,
	}, nil
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	oCfg := *(cfg.(*Config))
	exp, err := newMetricsExporter(oCfg, set, metricsMarshalers())
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exp.metricsDataPusher,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable exporterhelper Timeout, because we cannot pass a Context to the Producer,
		// and will rely on the sarama Pulsar Timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(oCfg.RetrySettings),
		exporterhelper.WithQueue(oCfg.QueueSettings),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Close))
}
