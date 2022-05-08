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

package pulsarreceiver

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
)

const (
	typeStr             = "pulsar"
	defaultEncoding     = "otlp_proto"
	defaultTopic        = "otlp_spans"
	defaultConsumerName = ""
	defaultSubscription = "otlp_subscription"
	defaultServiceUrl   = "pulsar://localhost:6650"
)

// FactoryOption applies changes to PulsarExporterFactory.
type FactoryOption func(factory *PulsarReceiverFactory)

// WithTracesUnmarshalers adds Unmarshalers.
func WithTracesUnmarshalers(tracesUnmarshalers ...TracesUnmarshaler) FactoryOption {
	return func(factory *PulsarReceiverFactory) {
		for _, unmarshaler := range tracesUnmarshalers {
			factory.tracesUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// WithMetricsUnmarshalers adds MetricsUnmarshalers.
func WithMetricsUnmarshalers(metricsUnmarshalers ...MetricsUnmarshaler) FactoryOption {
	return func(factory *PulsarReceiverFactory) {
		for _, unmarshaler := range metricsUnmarshalers {
			factory.metricsUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// WithLogsUnmarshalers adds LogsUnmarshalers.
func WithLogsUnmarshalers(logsUnmarshalers ...LogsUnmarshaler) FactoryOption {
	return func(factory *PulsarReceiverFactory) {
		for _, unmarshaler := range logsUnmarshalers {
			factory.logsUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// NewFactory creates Kafka receiver factory.
func NewFactory(options ...FactoryOption) component.ReceiverFactory {

	f := &PulsarReceiverFactory{
		tracesUnmarshalers:  defaultTracesUnmarshalers(),
		metricsUnmarshalers: defaultMetricsUnmarshalers(),
		logsUnmarshalers:    defaultLogsUnmarshalers(),
	}
	for _, o := range options {
		o(f)
	}
	return component.NewReceiverFactory(
		typeStr,
		createDefaultConfig,
		component.WithTracesReceiver(f.createTracesReceiver),
		component.WithMetricsReceiver(f.createMetricsReceiver),
		component.WithLogsReceiver(f.createLogsReceiver),
	)
}

type PulsarReceiverFactory struct {
	tracesUnmarshalers  map[string]TracesUnmarshaler
	metricsUnmarshalers map[string]MetricsUnmarshaler
	logsUnmarshalers    map[string]LogsUnmarshaler
}

func (f *PulsarReceiverFactory) createTracesReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Traces,
) (component.TracesReceiver, error) {
	c := cfg.(*Config)
	r, err := newTracesReceiver(*c, set, f.tracesUnmarshalers, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *PulsarReceiverFactory) createMetricsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	c := cfg.(*Config)
	r, err := newMetricsReceiver(*c, set, f.metricsUnmarshalers, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *PulsarReceiverFactory) createLogsReceiver(
	_ context.Context,
	set component.ReceiverCreateSettings,
	cfg config.Receiver,
	nextConsumer consumer.Logs,
) (component.LogsReceiver, error) {
	c := cfg.(*Config)
	r, err := newLogsReceiver(*c, set, f.logsUnmarshalers, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func createDefaultConfig() config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		Topic:            defaultTopic,
		Encoding:         defaultEncoding,
		ConsumerName:     defaultConsumerName,
		Subscription:     defaultSubscription,
		ServiceUrl:       defaultServiceUrl,
	}
}
