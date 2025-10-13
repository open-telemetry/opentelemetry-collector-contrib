// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"fmt"
	"iter"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

const franzGoClientFeatureGateName = "exporter.kafkaexporter.UseFranzGo"

// franzGoClientFeatureGate is a feature gate that controls whether the Kafka exporter
// uses the franz-go client or the Sarama client. When enabled, the Kafka exporter
// will use the franz-go client, which is more performant and has better support for
// modern Kafka features.
var franzGoClientFeatureGate = featuregate.GlobalRegistry().MustRegister(
	franzGoClientFeatureGateName, featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, the Kafka exporter will use the franz-go client to produce messages to Kafka."),
	featuregate.WithRegisterFromVersion("v0.128.0"),
)

// producer is an interface that abstracts the Kafka producer operations
// to allow for different implementations (e.g., Sarama, franz-go)
type producer interface {
	// ExportData sends a batch of messages to Kafka
	ExportData(ctx context.Context, messages kafkaclient.Messages) error
	// Close shuts down the producer
	Close() error
}

type messenger[T any] interface {
	// partitionData returns an iterator that yields key-value pairs
	// where the key is the partition key, and the value is the pdata
	// type (plog.Logs, etc.)
	partitionData(T) iter.Seq2[[]byte, T]

	// marshalData marshals a pdata type into one or more messages.
	marshalData(T) ([]marshaler.Message, error)

	// getTopic returns the topic name for the given context and data.
	getTopic(context.Context, T) string
}

type kafkaExporter[T any] struct {
	cfg          Config
	set          exporter.Settings
	tb           *metadata.TelemetryBuilder
	logger       *zap.Logger
	newMessenger func(host component.Host) (messenger[T], error)
	messenger    messenger[T]
	producer     producer
}

func newKafkaExporter[T any](
	config Config,
	set exporter.Settings,
	newMessenger func(component.Host) (messenger[T], error),
) *kafkaExporter[T] {
	return &kafkaExporter[T]{
		cfg:          config,
		set:          set,
		logger:       set.Logger,
		newMessenger: newMessenger,
	}
}

func (e *kafkaExporter[T]) Start(ctx context.Context, host component.Host) (err error) {
	tb, err := metadata.NewTelemetryBuilder(e.set.TelemetrySettings)
	if err != nil {
		return err
	}
	e.tb = tb

	if e.messenger, err = e.newMessenger(host); err != nil {
		return err
	}

	if franzGoClientFeatureGate.IsEnabled() {
		producer, ferr := kafka.NewFranzSyncProducer(
			ctx,
			e.cfg.ClientConfig,
			e.cfg.Producer,
			e.cfg.TimeoutSettings.Timeout,
			e.logger,
			kgo.WithHooks(kafkaclient.NewFranzProducerMetrics(tb)),
		)
		if ferr != nil {
			return ferr
		}
		e.producer = kafkaclient.NewFranzSyncProducer(producer,
			e.cfg.IncludeMetadataKeys,
		)
		return nil
	}
	producer, err := kafka.NewSaramaSyncProducer(ctx, e.cfg.ClientConfig,
		e.cfg.Producer, e.cfg.TimeoutSettings.Timeout,
	)
	if err != nil {
		return err
	}
	e.producer = kafkaclient.NewSaramaSyncProducer(
		producer,
		kafkaclient.NewSaramaProducerMetrics(tb),
		e.cfg.IncludeMetadataKeys,
	)
	return nil
}

func (e *kafkaExporter[T]) Close(context.Context) (err error) {
	if e.producer == nil {
		return nil
	}
	err = e.producer.Close()
	e.producer = nil
	if e.tb != nil {
		e.tb.Shutdown()
		e.tb = nil
	}
	return err
}

func (e *kafkaExporter[T]) exportData(ctx context.Context, data T) error {
	var m kafkaclient.Messages
	for key, data := range e.messenger.partitionData(data) {
		topic := e.messenger.getTopic(ctx, data)
		partitionMessages, err := e.messenger.marshalData(data)
		if err != nil {
			err = fmt.Errorf("error exporting to topic %q: %w", topic, err)
			e.logger.Error("kafka records marshal data failed",
				zap.String("topic", topic),
				zap.Error(err),
			)
			return consumererror.NewPermanent(err)
		}
		for i := range partitionMessages {
			// Marshalers may set the Key, so don't override
			// if it's set and we're not partitioning here.
			if key != nil {
				partitionMessages[i].Key = key
			}
		}
		m.Count += len(partitionMessages)
		m.TopicMessages = append(m.TopicMessages, kafkaclient.TopicMessages{
			Topic:    topic,
			Messages: partitionMessages,
		})
	}
	err := e.producer.ExportData(ctx, m)
	if err == nil {
		if e.logger.Core().Enabled(zap.DebugLevel) {
			for _, mi := range m.TopicMessages {
				e.logger.Debug("kafka records exported",
					zap.Int("records", len(mi.Messages)),
					zap.String("topic", mi.Topic),
				)
			}
		}
	} else {
		for _, mi := range m.TopicMessages {
			e.logger.Error("kafka records export failed",
				zap.Int("records", len(mi.Messages)),
				zap.String("topic", mi.Topic),
				zap.Error(err),
			)
		}
	}
	return err
}

func newTracesExporter(config Config, set exporter.Settings) *kafkaExporter[ptrace.Traces] {
	// Jaeger encodings do their own partitioning, so disable trace ID
	// partitioning when they are configured.
	switch config.Traces.Encoding {
	case "jaeger_proto", "jaeger_json":
		config.PartitionTracesByID = false
	}
	return newKafkaExporter(config, set, func(host component.Host) (messenger[ptrace.Traces], error) {
		marshaler, err := getTracesMarshaler(config.Traces.Encoding, host)
		if err != nil {
			return nil, err
		}
		return &kafkaTracesMessenger{
			config:    config,
			marshaler: marshaler,
		}, nil
	})
}

type kafkaTracesMessenger struct {
	config    Config
	marshaler marshaler.TracesMarshaler
}

func (e *kafkaTracesMessenger) marshalData(td ptrace.Traces) ([]marshaler.Message, error) {
	return e.marshaler.MarshalTraces(td)
}

func (e *kafkaTracesMessenger) getTopic(ctx context.Context, td ptrace.Traces) string {
	return getTopic[ptrace.ResourceSpans](ctx, e.config.Traces, e.config.TopicFromAttribute, td.ResourceSpans())
}

func (e *kafkaTracesMessenger) partitionData(td ptrace.Traces) iter.Seq2[[]byte, ptrace.Traces] {
	return func(yield func([]byte, ptrace.Traces) bool) {
		if !e.config.PartitionTracesByID {
			yield(nil, td)
			return
		}
		for _, td := range batchpersignal.SplitTraces(td) {
			// Note that batchpersignal.SplitTraces guarantees that each trace
			// has exactly one trace, and by implication, at least one span.
			key := []byte(traceutil.TraceIDToHexOrEmptyString(
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID(),
			))
			if !yield(key, td) {
				return
			}
		}
	}
}

func newLogsExporter(config Config, set exporter.Settings) *kafkaExporter[plog.Logs] {
	return newKafkaExporter(config, set, func(host component.Host) (messenger[plog.Logs], error) {
		marshaler, err := getLogsMarshaler(config.Logs.Encoding, host)
		if err != nil {
			return nil, err
		}
		return &kafkaLogsMessenger{
			config:    config,
			marshaler: marshaler,
		}, nil
	})
}

type kafkaLogsMessenger struct {
	config    Config
	marshaler marshaler.LogsMarshaler
}

func (e *kafkaLogsMessenger) marshalData(ld plog.Logs) ([]marshaler.Message, error) {
	return e.marshaler.MarshalLogs(ld)
}

func (e *kafkaLogsMessenger) getTopic(ctx context.Context, ld plog.Logs) string {
	return getTopic[plog.ResourceLogs](ctx, e.config.Logs, e.config.TopicFromAttribute, ld.ResourceLogs())
}

func (e *kafkaLogsMessenger) partitionData(ld plog.Logs) iter.Seq2[[]byte, plog.Logs] {
	return func(yield func([]byte, plog.Logs) bool) {
		if e.config.PartitionLogsByResourceAttributes {
			for _, resourceLogs := range ld.ResourceLogs().All() {
				hash := pdatautil.MapHash(resourceLogs.Resource().Attributes())
				newLogs := plog.NewLogs()
				resourceLogs.CopyTo(newLogs.ResourceLogs().AppendEmpty())
				if !yield(hash[:], newLogs) {
					return
				}
			}
			return
		}
		if e.config.PartitionLogsByTraceID {
			for _, l := range batchpersignal.SplitLogs(ld) {
				var key []byte
				traceID := l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID()
				if !traceID.IsEmpty() {
					key = []byte(traceutil.TraceIDToHexOrEmptyString(traceID))
				}
				if !yield(key, l) {
					return
				}
			}
			return
		}
		yield(nil, ld)
	}
}

func newMetricsExporter(config Config, set exporter.Settings) *kafkaExporter[pmetric.Metrics] {
	return newKafkaExporter(config, set, func(host component.Host) (messenger[pmetric.Metrics], error) {
		marshaler, err := getMetricsMarshaler(config.Metrics.Encoding, host)
		if err != nil {
			return nil, err
		}
		return &kafkaMetricsMessenger{
			config:    config,
			marshaler: marshaler,
		}, nil
	})
}

type kafkaMetricsMessenger struct {
	config    Config
	marshaler marshaler.MetricsMarshaler
}

func (e *kafkaMetricsMessenger) marshalData(md pmetric.Metrics) ([]marshaler.Message, error) {
	return e.marshaler.MarshalMetrics(md)
}

func (e *kafkaMetricsMessenger) getTopic(ctx context.Context, md pmetric.Metrics) string {
	return getTopic[pmetric.ResourceMetrics](ctx, e.config.Metrics, e.config.TopicFromAttribute, md.ResourceMetrics())
}

func (e *kafkaMetricsMessenger) partitionData(md pmetric.Metrics) iter.Seq2[[]byte, pmetric.Metrics] {
	return func(yield func([]byte, pmetric.Metrics) bool) {
		if !e.config.PartitionMetricsByResourceAttributes {
			yield(nil, md)
			return
		}
		for _, resourceMetrics := range md.ResourceMetrics().All() {
			hash := pdatautil.MapHash(resourceMetrics.Resource().Attributes())
			newMetrics := pmetric.NewMetrics()
			resourceMetrics.CopyTo(newMetrics.ResourceMetrics().AppendEmpty())
			if !yield(hash[:], newMetrics) {
				return
			}
		}
	}
}

func newProfilesExporter(config Config, set exporter.Settings) *kafkaExporter[pprofile.Profiles] {
	return newKafkaExporter(config, set, func(host component.Host) (messenger[pprofile.Profiles], error) {
		marshaler, err := getProfilesMarshaler(config.Profiles.Encoding, host)
		if err != nil {
			return nil, err
		}
		return &kafkaProfilesMessenger{
			config:    config,
			marshaler: marshaler,
		}, nil
	})
}

type kafkaProfilesMessenger struct {
	config    Config
	marshaler marshaler.ProfilesMarshaler
}

func (e *kafkaProfilesMessenger) marshalData(ld pprofile.Profiles) ([]marshaler.Message, error) {
	return e.marshaler.MarshalProfiles(ld)
}

func (e *kafkaProfilesMessenger) getTopic(ctx context.Context, ld pprofile.Profiles) string {
	return getTopic[pprofile.ResourceProfiles](ctx, e.config.Profiles, e.config.TopicFromAttribute, ld.ResourceProfiles())
}

func (*kafkaProfilesMessenger) partitionData(ld pprofile.Profiles) iter.Seq2[[]byte, pprofile.Profiles] {
	return func(yield func([]byte, pprofile.Profiles) bool) {
		yield(nil, ld)
	}
}

type resourceSlice[T any] interface {
	Len() int
	At(int) T
}

type resource interface {
	Resource() pcommon.Resource
}

func getTopic[T resource](ctx context.Context,
	signalCfg SignalConfig,
	topicFromAttribute string,
	resources resourceSlice[T],
) string {
	if k := signalCfg.TopicFromMetadataKey; k != "" {
		if topic := client.FromContext(ctx).Metadata.Get(k); len(topic) > 0 {
			return topic[0]
		}
	}
	if topicFromAttribute != "" {
		for i := 0; i < resources.Len(); i++ {
			rv, ok := resources.At(i).Resource().Attributes().Get(topicFromAttribute)
			if ok && rv.Str() != "" {
				return rv.Str()
			}
		}
	}
	if topic, ok := topic.FromContext(ctx); ok {
		return topic
	}
	return signalCfg.Topic
}
