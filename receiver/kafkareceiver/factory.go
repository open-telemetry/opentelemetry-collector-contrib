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

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"fmt"
	"time"

	"go.opencensus.io/stats/view"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
)

const (
	typeStr   = "kafka"
	stability = component.StabilityLevelBeta

	defaultTopic    = "otlp_spans"
	defaultEncoding = "otlp_proto"
	defaultBroker   = "localhost:9092"
	defaultClientID = "otel-collector"
	defaultGroupID  = defaultClientID

	// default from sarama.NewConfig()
	defaultMetadataRetryMax = 3
	// default from sarama.NewConfig()
	defaultMetadataRetryBackoff = time.Millisecond * 250
	// default from sarama.NewConfig()
	defaultMetadataFull = true

	// default from sarama.NewConfig()
	defaultAutoCommitEnable = true
	// default from sarama.NewConfig()
	defaultAutoCommitInterval = 1 * time.Second
)

var errUnrecognizedEncoding = fmt.Errorf("unrecognized encoding")

// FactoryOption applies changes to kafkaExporterFactory.
type FactoryOption func(factory *kafkaReceiverFactory)

// WithTracesUnmarshalers adds Unmarshalers.
func WithTracesUnmarshalers(tracesUnmarshalers ...TracesUnmarshaler) FactoryOption {
	return func(factory *kafkaReceiverFactory) {
		for _, unmarshaler := range tracesUnmarshalers {
			factory.tracesUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// WithMetricsUnmarshalers adds MetricsUnmarshalers.
func WithMetricsUnmarshalers(metricsUnmarshalers ...MetricsUnmarshaler) FactoryOption {
	return func(factory *kafkaReceiverFactory) {
		for _, unmarshaler := range metricsUnmarshalers {
			factory.metricsUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// WithLogsUnmarshalers adds LogsUnmarshalers.
func WithLogsUnmarshalers(logsUnmarshalers ...LogsUnmarshaler) FactoryOption {
	return func(factory *kafkaReceiverFactory) {
		for _, unmarshaler := range logsUnmarshalers {
			factory.logsUnmarshalers[unmarshaler.Encoding()] = unmarshaler
		}
	}
}

// NewFactory creates Kafka receiver factory.
func NewFactory(options ...FactoryOption) receiver.Factory {
	_ = view.Register(MetricViews()...)

	f := &kafkaReceiverFactory{
		tracesUnmarshalers:  map[string]TracesUnmarshaler{},
		metricsUnmarshalers: map[string]MetricsUnmarshaler{},
		logsUnmarshalers:    map[string]LogsUnmarshaler{},
	}

	for _, o := range options {
		o(f)
	}

	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithTraces(f.createTracesReceiver, stability),
		receiver.WithMetrics(f.createMetricsReceiver, stability),
		receiver.WithLogs(f.createLogsReceiver, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Topic:    defaultTopic,
		Encoding: defaultEncoding,
		Brokers:  []string{defaultBroker},
		ClientID: defaultClientID,
		GroupID:  defaultGroupID,
		Metadata: kafkaexporter.Metadata{
			Full: defaultMetadataFull,
			Retry: kafkaexporter.MetadataRetry{
				Max:     defaultMetadataRetryMax,
				Backoff: defaultMetadataRetryBackoff,
			},
		},
		AutoCommit: AutoCommit{
			Enable:   defaultAutoCommitEnable,
			Interval: defaultAutoCommitInterval,
		},
		MessageMarking: MessageMarking{
			After:   false,
			OnError: false,
		},
	}
}

type kafkaReceiverFactory struct {
	tracesUnmarshalers  map[string]TracesUnmarshaler
	metricsUnmarshalers map[string]MetricsUnmarshaler
	logsUnmarshalers    map[string]LogsUnmarshaler
}

func (f *kafkaReceiverFactory) createTracesReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	for encoding, unmarshal := range defaultTracesUnmarshalers() {
		f.tracesUnmarshalers[encoding] = unmarshal
	}

	c := cfg.(*Config)
	unmarshaler := f.tracesUnmarshalers[c.Encoding]
	if unmarshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	r, err := newTracesReceiver(*c, set, unmarshaler, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *kafkaReceiverFactory) createMetricsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	for encoding, unmarshal := range defaultMetricsUnmarshalers() {
		f.metricsUnmarshalers[encoding] = unmarshal
	}

	c := cfg.(*Config)
	unmarshaler := f.metricsUnmarshalers[c.Encoding]
	if unmarshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	r, err := newMetricsReceiver(*c, set, unmarshaler, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *kafkaReceiverFactory) createLogsReceiver(
	_ context.Context,
	set receiver.CreateSettings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	for encoding, unmarshal := range defaultLogsUnmarshalers(set.BuildInfo.Version, set.Logger) {
		f.logsUnmarshalers[encoding] = unmarshal
	}

	c := cfg.(*Config)
	unmarshaler := f.logsUnmarshalers[c.Encoding]
	if unmarshaler == nil {
		return nil, errUnrecognizedEncoding
	}

	r, err := newLogsReceiver(*c, set, unmarshaler, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}
