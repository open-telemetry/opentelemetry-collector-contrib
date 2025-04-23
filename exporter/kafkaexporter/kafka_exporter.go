// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

type kafkaErrors struct {
	count int
	err   string
}

func (ke kafkaErrors) Error() string {
	return fmt.Sprintf("Failed to deliver %d messages due to %s", ke.count, ke.err)
}

type kafkaMessager[T any] interface {
	// partitionData returns an iterator that yields key-value pairs
	// where the key is the partition key, and the value is the pdata
	// type (plog.Logs, etc.)
	partitionData(T) iter.Seq2[[]byte, T]

	// marshalData marshals a pdata type into onr or more messages.
	marshalData(T) ([]marshaler.Message, error)

	// getTopic returns the topic name for the given context and data.
	getTopic(context.Context, T) string
}

type kafkaExporter[T any] struct {
	cfg         Config
	logger      *zap.Logger
	newMessager func(host component.Host) (kafkaMessager[T], error)

	messager kafkaMessager[T]
	producer sarama.SyncProducer
}

func newKafkaExporter[T any](
	config Config,
	set exporter.Settings,
	newMessager func(component.Host) (kafkaMessager[T], error),
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
	e.messager = messager

	producer, err := kafka.NewSaramaSyncProducer(
		ctx, e.cfg.ClientConfig, e.cfg.Producer,
		e.cfg.TimeoutSettings.Timeout,
	)
	if err != nil {
		return err
	}
	e.producer = producer
	return nil
}

func (e *kafkaExporter[T]) Close(context.Context) error {
	if e.producer == nil {
		return nil
	}
	return e.producer.Close()
}

func (e *kafkaExporter[T]) exportData(ctx context.Context, data T) error {
	var allSaramaMessages []*sarama.ProducerMessage
	for key, data := range e.messager.partitionData(data) {
		partitionMessages, err := e.messager.marshalData(data)
		if err != nil {
			return consumererror.NewPermanent(err)
		}
		for i := range partitionMessages {
			// Marshalers may set the Key, so don't override
			// if it's set and we're not partitioning here.
			if key != nil {
				partitionMessages[i].Key = key
			}
		}
		saramaMessages := makeSaramaMessages(partitionMessages, e.messager.getTopic(ctx, data))
		allSaramaMessages = append(allSaramaMessages, saramaMessages...)
	}
	messagesWithHeaders(allSaramaMessages, metadataToHeaders(
		ctx, e.cfg.IncludeMetadataKeys,
	))
	if err := e.producer.SendMessages(allSaramaMessages); err != nil {
		var prodErr sarama.ProducerErrors
		if errors.As(err, &prodErr) {
			if len(prodErr) > 0 {
				return kafkaErrors{len(prodErr), prodErr[0].Err.Error()}
			}
		}
		return err
	}
	return nil
}

func newTracesExporter(config Config, set exporter.Settings) *kafkaExporter[ptrace.Traces] {
	// Jaeger encodings do their own partitioning, so disable trace ID
	// partitioning when they are configured.
	switch config.Traces.Encoding {
	case "jaeger_proto", "jaeger_json":
		config.PartitionTracesByID = false
	}
	return newKafkaExporter(config, set, func(host component.Host) (kafkaMessager[ptrace.Traces], error) {
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

func (e *kafkaTracesMessager) marshalData(td ptrace.Traces) ([]marshaler.Message, error) {
	return e.marshaler.MarshalTraces(td)
}

func (e *kafkaTracesMessager) getTopic(ctx context.Context, td ptrace.Traces) string {
	return getTopic(ctx, &e.config, td.ResourceSpans())
}

func (e *kafkaTracesMessager) partitionData(td ptrace.Traces) iter.Seq2[[]byte, ptrace.Traces] {
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
	return newKafkaExporter(config, set, func(host component.Host) (kafkaMessager[plog.Logs], error) {
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

func (e *kafkaLogsMessager) marshalData(ld plog.Logs) ([]marshaler.Message, error) {
	return e.marshaler.MarshalLogs(ld)
}

func (e *kafkaLogsMessager) getTopic(ctx context.Context, ld plog.Logs) string {
	return getTopic(ctx, &e.config, ld.ResourceLogs())
}

func (e *kafkaLogsMessager) partitionData(ld plog.Logs) iter.Seq2[[]byte, plog.Logs] {
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
	return newKafkaExporter(config, set, func(host component.Host) (kafkaMessager[pmetric.Metrics], error) {
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

func (e *kafkaMetricsMessager) marshalData(md pmetric.Metrics) ([]marshaler.Message, error) {
	return e.marshaler.MarshalMetrics(md)
}

func (e *kafkaMetricsMessager) getTopic(ctx context.Context, md pmetric.Metrics) string {
	return getTopic(ctx, &e.config, md.ResourceMetrics())
}

func (e *kafkaMetricsMessager) partitionData(md pmetric.Metrics) iter.Seq2[[]byte, pmetric.Metrics] {
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

func getTopic[T resource](ctx context.Context, cfg *Config, resources resourceSlice[T]) string {
	if cfg.TopicFromAttribute != "" {
		for i := 0; i < resources.Len(); i++ {
			rv, ok := resources.At(i).Resource().Attributes().Get(cfg.TopicFromAttribute)
			if ok && rv.Str() != "" {
				return rv.Str()
			}
		}
	}
	contextTopic, ok := topic.FromContext(ctx)
	if ok {
		return contextTopic
	}
	return cfg.Topic
}

func makeSaramaMessages(messages []marshaler.Message, topic string) []*sarama.ProducerMessage {
	saramaMessages := make([]*sarama.ProducerMessage, len(messages))
	for i, message := range messages {
		saramaMessages[i] = &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.ByteEncoder(message.Key),
			Value: sarama.ByteEncoder(message.Value),
		}
	}
	return saramaMessages
}

func messagesWithHeaders(msg []*sarama.ProducerMessage, h []sarama.RecordHeader) {
	if len(h) == 0 || len(msg) == 0 {
		return
	}
	for i := range msg {
		if len(msg[i].Headers) == 0 {
			msg[i].Headers = h
			continue
		}
		msg[i].Headers = append(msg[i].Headers, h...)
	}
}

func metadataToHeaders(ctx context.Context, keys []string) []sarama.RecordHeader {
	if len(keys) == 0 {
		return nil
	}
	info := client.FromContext(ctx)
	headers := make([]sarama.RecordHeader, 0, len(keys))
	for _, key := range keys {
		valueSlice := info.Metadata.Get(key)
		for _, v := range valueSlice {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte(key),
				Value: []byte(v),
			})
		}
	}
	return headers
}
