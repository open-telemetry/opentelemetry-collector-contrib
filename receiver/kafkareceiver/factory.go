// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

const (
	defaultTracesTopic       = "otlp_spans"
	defaultMetricsTopic      = "otlp_metrics"
	defaultLogsTopic         = "otlp_logs"
	defaultFormatType        = "otlp_proto"
	defaultBroker            = "localhost:9092"
	defaultClientID          = "otel-collector"
	defaultGroupID           = defaultClientID
	defaultInitialOffset     = offsetLatest
	defaultSessionTimeout    = 10 * time.Second
	defaultHeartbeatInterval = 3 * time.Second

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

var errUnrecognizedFormatType = fmt.Errorf("unrecognized format")

// FactoryOption applies changes to kafkaExporterFactory.
type FactoryOption func(factory *kafkaReceiverFactory)

// withTracesUnmarshalers adds Unmarshalers.
func withTracesUnmarshalers(tracesUnmarshalers ...TracesUnmarshaler) FactoryOption {
	return func(factory *kafkaReceiverFactory) {
		for _, unmarshaler := range tracesUnmarshalers {
			factory.tracesUnmarshalers[unmarshaler.FormatType()] = unmarshaler
		}
	}
}

// withMetricsUnmarshalers adds MetricsUnmarshalers.
func withMetricsUnmarshalers(metricsUnmarshalers ...MetricsUnmarshaler) FactoryOption {
	return func(factory *kafkaReceiverFactory) {
		for _, unmarshaler := range metricsUnmarshalers {
			factory.metricsUnmarshalers[unmarshaler.FormatType()] = unmarshaler
		}
	}
}

// withLogsUnmarshalers adds LogsUnmarshalers.
func withLogsUnmarshalers(logsUnmarshalers ...LogsUnmarshaler) FactoryOption {
	return func(factory *kafkaReceiverFactory) {
		for _, unmarshaler := range logsUnmarshalers {
			factory.logsUnmarshalers[unmarshaler.FormatType()] = unmarshaler
		}
	}
}

// NewFactory creates Kafka receiver factory.
func NewFactory(options ...FactoryOption) receiver.Factory {
	f := &kafkaReceiverFactory{
		tracesUnmarshalers:  map[string]TracesUnmarshaler{},
		metricsUnmarshalers: map[string]MetricsUnmarshaler{},
		logsUnmarshalers:    map[string]LogsUnmarshaler{},
	}
	for _, o := range options {
		o(f)
	}
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithTraces(f.createTracesReceiver, metadata.TracesStability),
		receiver.WithMetrics(f.createMetricsReceiver, metadata.MetricsStability),
		receiver.WithLogs(f.createLogsReceiver, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Encoding:          nil,
		FormatType:        defaultFormatType,
		Brokers:           []string{defaultBroker},
		ClientID:          defaultClientID,
		GroupID:           defaultGroupID,
		InitialOffset:     defaultInitialOffset,
		SessionTimeout:    defaultSessionTimeout,
		HeartbeatInterval: defaultHeartbeatInterval,
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
		HeaderExtraction: HeaderExtraction{
			ExtractHeaders: false,
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
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	for formatType, unmarshal := range defaultTracesUnmarshalers() {
		f.tracesUnmarshalers[formatType] = unmarshal
	}

	oCfg := *(cfg.(*Config))
	if oCfg.Topic == "" {
		oCfg.Topic = defaultTracesTopic
	}
	unmarshaler := f.tracesUnmarshalers[oCfg.FormatType]
	if unmarshaler == nil {
		return nil, errUnrecognizedFormatType
	}

	r, err := newTracesReceiver(oCfg, set, unmarshaler, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *kafkaReceiverFactory) createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	for formatType, unmarshal := range defaultMetricsUnmarshalers() {
		f.metricsUnmarshalers[formatType] = unmarshal
	}

	oCfg := *(cfg.(*Config))
	if oCfg.Topic == "" {
		oCfg.Topic = defaultMetricsTopic
	}
	unmarshaler := f.metricsUnmarshalers[oCfg.FormatType]
	if unmarshaler == nil {
		return nil, errUnrecognizedFormatType
	}

	r, err := newMetricsReceiver(oCfg, set, unmarshaler, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (f *kafkaReceiverFactory) createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	for formatType, unmarshaler := range defaultLogsUnmarshalers(set.BuildInfo.Version, set.Logger) {
		f.logsUnmarshalers[formatType] = unmarshaler
	}

	oCfg := *(cfg.(*Config))
	if oCfg.Topic == "" {
		oCfg.Topic = defaultLogsTopic
	}
	unmarshaler, err := getLogsUnmarshaler(oCfg.FormatType, f.logsUnmarshalers)
	if err != nil {
		return nil, err
	}

	r, err := newLogsReceiver(oCfg, set, unmarshaler, nextConsumer)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func getLogsUnmarshaler(formatType string, unmarshalers map[string]LogsUnmarshaler) (LogsUnmarshaler, error) {
	var enc string
	unmarshaler, ok := unmarshalers[formatType]
	if !ok {
		split := strings.SplitN(formatType, "_", 2)
		prefix := split[0]
		if len(split) > 1 {
			enc = split[1]
		}
		unmarshaler, ok = unmarshalers[prefix].(LogsUnmarshalerWithEnc)
		if !ok {
			return nil, errUnrecognizedFormatType
		}
	}

	if unmarshalerWithEnc, ok := unmarshaler.(LogsUnmarshalerWithEnc); ok {
		// This should be called even when enc is an empty string to initialize the format.
		unmarshaler, err := unmarshalerWithEnc.WithEnc(enc)
		if err != nil {
			return nil, err
		}
		return unmarshaler, nil
	}

	return unmarshaler, nil
}
