// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"iter"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

const (
	clientTypeFranzGo = "franz-go"
	clientTypeSarama  = "sarama"
)

// Producer is an interface that abstracts the Kafka producer operations
// to allow for different implementations (e.g., Sarama, franz-go)
type Producer[T any] interface {
	// ExportData sends a batch of messages to Kafka
	ExportData(ctx context.Context, data T) error
	// Close shuts down the producer
	Close() error
}

type kafkaExporter[T any] struct {
	cfg         Config
	logger      *zap.Logger
	newMessager func(host component.Host) (kafkaclient.Messenger[T], error)
	producer    Producer[T]
}

func newKafkaExporter[T any](
	config Config,
	set exporter.Settings,
	newMessager func(component.Host) (kafkaclient.Messenger[T], error),
) *kafkaExporter[T] {
	return &kafkaExporter[T]{
		cfg:         config,
		logger:      set.Logger,
		newMessager: newMessager,
	}
}

func (e *kafkaExporter[T]) Start(ctx context.Context, host component.Host) error {
	messager, err := e.newMessager(host)
	if err != nil {
		return err
	}

	switch e.cfg.ClientType {
	case clientTypeFranzGo:
		producer, err := kafka.NewFranzSyncProducer(e.cfg.ClientConfig,
			e.cfg.Producer, e.cfg.TimeoutSettings.Timeout, e.logger,
		)
		if err != nil {
			return err
		}
		e.producer = kafkaclient.NewFranzSyncProducer(producer, messager,
			e.cfg.IncludeMetadataKeys,
		)
	case clientTypeSarama:
		producer, err := kafka.NewSaramaSyncProducer(ctx, e.cfg.ClientConfig,
			e.cfg.Producer, e.cfg.TimeoutSettings.Timeout,
		)
		if err != nil {
			return err
		}
		e.producer = kafkaclient.NewSaramaSyncProducer(producer, messager,
			e.cfg.IncludeMetadataKeys,
		)
	}
	return nil
}

func (e *kafkaExporter[T]) Close(context.Context) (err error) {
	if e.producer == nil {
		return nil
	}
	err = e.producer.Close()
	e.producer = nil
	return err
}

func (e *kafkaExporter[T]) exportData(ctx context.Context, data T) error {
	return e.producer.ExportData(ctx, data)
}

func newTracesExporter(config Config, set exporter.Settings) *kafkaExporter[ptrace.Traces] {
	// Jaeger encodings do their own partitioning, so disable trace ID
	// partitioning when they are configured.
	switch config.Traces.Encoding {
	case "jaeger_proto", "jaeger_json":
		config.PartitionTracesByID = false
	}
	return newKafkaExporter(config, set, func(host component.Host) (kafkaclient.Messenger[ptrace.Traces], error) {
		marshaler, err := getTracesMarshaler(config.Traces.Encoding, host)
		if err != nil {
			return nil, err
		}
		return &kafkaTracesMessager{
			config:    config,
			marshaler: marshaler,
		}, nil
	})
}

type kafkaTracesMessager struct {
	config    Config
	marshaler marshaler.TracesMarshaler
}

func (e *kafkaTracesMessager) MarshalData(td ptrace.Traces) ([]marshaler.Message, error) {
	return e.marshaler.MarshalTraces(td)
}

func (e *kafkaTracesMessager) GetTopic(ctx context.Context, td ptrace.Traces) string {
	return getTopic(ctx, e.config.Traces, e.config.TopicFromAttribute, td.ResourceSpans())
}

func (e *kafkaTracesMessager) PartitionData(td ptrace.Traces) iter.Seq2[[]byte, ptrace.Traces] {
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
	return newKafkaExporter(config, set, func(host component.Host) (kafkaclient.Messenger[plog.Logs], error) {
		marshaler, err := getLogsMarshaler(config.Logs.Encoding, host)
		if err != nil {
			return nil, err
		}
		return &kafkaLogsMessager{
			config:    config,
			marshaler: marshaler,
		}, nil
	})
}

type kafkaLogsMessager struct {
	config    Config
	marshaler marshaler.LogsMarshaler
}

func (e *kafkaLogsMessager) MarshalData(ld plog.Logs) ([]marshaler.Message, error) {
	return e.marshaler.MarshalLogs(ld)
}

func (e *kafkaLogsMessager) GetTopic(ctx context.Context, ld plog.Logs) string {
	return getTopic(ctx, e.config.Logs, e.config.TopicFromAttribute, ld.ResourceLogs())
}

func (e *kafkaLogsMessager) PartitionData(ld plog.Logs) iter.Seq2[[]byte, plog.Logs] {
	return func(yield func([]byte, plog.Logs) bool) {
		if !e.config.PartitionLogsByResourceAttributes {
			yield(nil, ld)
			return
		}
		for _, resourceLogs := range ld.ResourceLogs().All() {
			hash := pdatautil.MapHash(resourceLogs.Resource().Attributes())
			newLogs := plog.NewLogs()
			resourceLogs.CopyTo(newLogs.ResourceLogs().AppendEmpty())
			if !yield(hash[:], newLogs) {
				return
			}
		}
	}
}

func newMetricsExporter(config Config, set exporter.Settings) *kafkaExporter[pmetric.Metrics] {
	return newKafkaExporter(config, set, func(host component.Host) (kafkaclient.Messenger[pmetric.Metrics], error) {
		marshaler, err := getMetricsMarshaler(config.Metrics.Encoding, host)
		if err != nil {
			return nil, err
		}
		return &kafkaMetricsMessager{
			config:    config,
			marshaler: marshaler,
		}, nil
	})
}

type kafkaMetricsMessager struct {
	config    Config
	marshaler marshaler.MetricsMarshaler
}

func (e *kafkaMetricsMessager) MarshalData(md pmetric.Metrics) ([]marshaler.Message, error) {
	return e.marshaler.MarshalMetrics(md)
}

func (e *kafkaMetricsMessager) GetTopic(ctx context.Context, md pmetric.Metrics) string {
	return getTopic(ctx, e.config.Metrics, e.config.TopicFromAttribute, md.ResourceMetrics())
}

func (e *kafkaMetricsMessager) PartitionData(md pmetric.Metrics) iter.Seq2[[]byte, pmetric.Metrics] {
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
