// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

const (
	defaultLogsTopic    = "otlp_logs"
	defaultLogsEncoding = "otlp_proto"

	defaultMetricsTopic    = "otlp_metrics"
	defaultMetricsEncoding = "otlp_proto"

	defaultTracesTopic    = "otlp_spans"
	defaultTracesEncoding = "otlp_proto"

	defaultProfilesTopic    = "otlp_profiles"
	defaultProfilesEncoding = "otlp_proto"
)

// NewFactory creates Kafka receiver factory.
func NewFactory() receiver.Factory {
	f := kafkaReceiverFactory{receivers: sharedcomponent.NewSharedComponents()}
	return xreceiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xreceiver.WithTraces(f.createTraces, metadata.TracesStability),
		xreceiver.WithMetrics(f.createMetrics, metadata.MetricsStability),
		xreceiver.WithLogs(f.createLogs, metadata.LogsStability),
		xreceiver.WithProfiles(f.createProfiles, metadata.ProfilesStability),
	)
}

type kafkaReceiverFactory struct {
	receivers *sharedcomponent.SharedComponents
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig:   configkafka.NewDefaultClientConfig(),
		ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
		Logs: TopicEncodingConfig{
			Topics:   []string{defaultLogsTopic},
			Encoding: defaultLogsEncoding,
		},
		Metrics: TopicEncodingConfig{
			Topics:   []string{defaultMetricsTopic},
			Encoding: defaultMetricsEncoding,
		},
		Traces: TopicEncodingConfig{
			Topics:   []string{defaultTracesTopic},
			Encoding: defaultTracesEncoding,
		},
		Profiles: TopicEncodingConfig{
			Topics:   []string{defaultProfilesTopic},
			Encoding: defaultProfilesEncoding,
		},
		MessageMarking: MessageMarking{
			After:            false,
			OnError:          false,
			OnPermanentError: false,
		},
		PartitionProcessing: PartitionProcessing{
			MaxBufferedBatches: 1,
		},
		HeaderExtraction: HeaderExtraction{
			ExtractHeaders: false,
		},
	}
}

// createShared reuses a muxReceiver per config when signal_header is
// enabled. Each Create* call registers that signal so all attached pipelines
// share one Kafka consumer.
func (f *kafkaReceiverFactory) createShared(cfg *Config,
	set receiver.Settings, register func(*muxReceiver) error,
) (*sharedcomponent.SharedComponent, error) {
	var err error
	shared := f.receivers.GetOrAdd(cfg, func() component.Component {
		var receiver *muxReceiver
		receiver, err = newMuxReceiver(cfg, set)
		return receiver
	})
	if err != nil {
		return nil, err
	}
	if err := register(shared.Unwrap().(*muxReceiver)); err != nil {
		return nil, err
	}
	return shared, nil
}

func (f *kafkaReceiverFactory) createTraces(ctx context.Context,
	set receiver.Settings, cfg component.Config, nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	config := cfg.(*Config)
	if !config.SignalHeader {
		return createTracesReceiver(ctx, set, cfg, nextConsumer)
	}
	return f.createShared(config, set, func(mux *muxReceiver) error {
		return mux.registerTraces(set, nextConsumer)
	})
}

func (f *kafkaReceiverFactory) createMetrics(ctx context.Context,
	set receiver.Settings, cfg component.Config, nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	config := cfg.(*Config)
	if !config.SignalHeader {
		return createMetricsReceiver(ctx, set, cfg, nextConsumer)
	}
	return f.createShared(config, set, func(mux *muxReceiver) error {
		return mux.registerMetrics(set, nextConsumer)
	})
}

func (f *kafkaReceiverFactory) createLogs(ctx context.Context,
	set receiver.Settings, cfg component.Config, nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	config := cfg.(*Config)
	if !config.SignalHeader {
		return createLogsReceiver(ctx, set, cfg, nextConsumer)
	}
	return f.createShared(config, set, func(mux *muxReceiver) error {
		return mux.registerLogs(set, nextConsumer)
	})
}

func (f *kafkaReceiverFactory) createProfiles(ctx context.Context,
	set receiver.Settings, cfg component.Config, nextConsumer xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	config := cfg.(*Config)
	if !config.SignalHeader {
		return createProfilesReceiver(ctx, set, cfg, nextConsumer)
	}
	return f.createShared(config, set, func(mux *muxReceiver) error {
		return mux.registerProfiles(set, nextConsumer)
	})
}

func createTracesReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Traces,
) (receiver.Traces, error) {
	return newTracesReceiver(cfg.(*Config), set, nextConsumer)
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (receiver.Metrics, error) {
	return newMetricsReceiver(cfg.(*Config), set, nextConsumer)
}

func createLogsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer consumer.Logs,
) (receiver.Logs, error) {
	return newLogsReceiver(cfg.(*Config), set, nextConsumer)
}

func createProfilesReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	nextConsumer xconsumer.Profiles,
) (xreceiver.Profiles, error) {
	return newProfilesReceiver(cfg.(*Config), set, nextConsumer)
}
