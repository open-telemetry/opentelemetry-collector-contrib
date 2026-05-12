// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
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

type messenger[T any] interface {
	// partitionData returns an iterator that yields key-value pairs
	// where the key is the partition key, and the value is the pdata
	// type (plog.Logs, etc.)
	partitionData(T) iter.Seq2[[]byte, T]

	// marshalData marshals a pdata type into zero or more messages,
	// invoking yield once per message with its key and value.
	marshalData(data T, yield func(key, value []byte)) error

	// getTopic returns the topic name for the given context and data.
	getTopic(context.Context, T) string
}

// recordsBuffer is a pooled holder for a batch of kgo.Records. space owns
// the record values; pointers[i] points to space[i] and is what the producer
// API expects. Both slices are reused across exports.
type recordsBuffer struct {
	space    []kgo.Record
	pointers []*kgo.Record
}

type kafkaExporter[T any] struct {
	cfg          Config
	set          exporter.Settings
	tb           *metadata.TelemetryBuilder
	logger       *zap.Logger
	newMessenger func(host component.Host) (messenger[T], error)
	messenger    messenger[T]
	producer     *kafkaclient.FranzSyncProducer
	recordsPool  sync.Pool
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
		recordsPool: sync.Pool{
			New: func() any {
				return &recordsBuffer{}
			},
		},
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

	partitionerOpt, err := buildPartitionerOpt(e.cfg.RecordPartitioner, host)
	if err != nil {
		return fmt.Errorf("failed to configure record partitioner: %w", err)
	}

	clientCtx, clientCancel := context.WithCancel(context.Background())
	producer, err := kafka.NewFranzSyncProducer(
		ctx,
		host,
		e.cfg.ClientConfig,
		e.cfg.Producer,
		e.cfg.TimeoutSettings.Timeout,
		e.logger,
		kgo.WithContext(clientCtx),
		kgo.WithHooks(kafkaclient.NewFranzProducerMetrics(tb)),
		partitionerOpt,
	)
	if err != nil {
		clientCancel()
		return err
	}
	e.producer = kafkaclient.NewFranzSyncProducer(producer,
		e.cfg.IncludeMetadataKeys,
		e.cfg.RecordHeaders,
		e.cfg.Producer.MaxMessageBytes,
		clientCancel,
	)
	return nil
}

func (e *kafkaExporter[T]) Close(ctx context.Context) (err error) {
	if e.tb != nil {
		e.tb.Shutdown()
		e.tb = nil
	}
	if e.producer == nil {
		return nil
	}
	err = e.producer.Close(ctx)
	e.producer = nil
	return err
}

func (e *kafkaExporter[T]) exportData(ctx context.Context, data T) error {
	buf := e.recordsPool.Get().(*recordsBuffer)
	buf.space = buf.space[:0]
	defer func() {
		clear(buf.space)
		clear(buf.pointers)
		e.recordsPool.Put(buf)
	}()
	for partitionKey, data := range e.messenger.partitionData(data) {
		topic := e.messenger.getTopic(ctx, data)
		err := e.messenger.marshalData(data, func(key, value []byte) {
			// Marshalers may set the key, but a non-nil partition key
			// from partitionData takes precedence.
			if partitionKey != nil {
				key = partitionKey
			}
			buf.space = append(buf.space, kgo.Record{
				Topic: topic,
				Key:   key,
				Value: value,
			})
		})
		if err != nil {
			err = fmt.Errorf("error exporting to topic %q: %w", topic, err)
			e.logger.Error("kafka records marshal data failed",
				zap.String("topic", topic),
				zap.Error(err),
			)
			return consumererror.NewPermanent(err)
		}
	}
	// Build the pointer slice from space. We do this once here rather
	// than in lockstep with each append, since append may reallocate
	// space's backing array and invalidate earlier pointers.
	buf.pointers = slices.Grow(buf.pointers[:0], len(buf.space))
	for i := range buf.space {
		buf.pointers = append(buf.pointers, &buf.space[i])
	}
	err := e.producer.ExportData(ctx, buf.pointers)
	if err != nil {
		e.logger.Error("kafka records export failed",
			zap.Int("records", len(buf.pointers)),
			zap.Error(err),
		)
		var msgTooLarge *kafkaclient.MessageTooLargeError
		if errors.As(err, &msgTooLarge) {
			e.logger.Error("kafka record exceeds max message size",
				zap.Int("actual_message_bytes", msgTooLarge.RecordBytes),
				zap.Int("max_message_bytes", msgTooLarge.MaxMessageBytes),
			)
		}
		return err
	}
	// TODO move this logging to a kgo hook, so we capture topic and partition details.
	if e.logger.Core().Enabled(zap.DebugLevel) {
		e.logger.Debug("kafka records exported",
			zap.Int("records", len(buf.pointers)),
		)
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

func (e *kafkaTracesMessenger) marshalData(td ptrace.Traces, yield func(key, value []byte)) error {
	return e.marshaler.MarshalTraces(td, yield)
}

func (e *kafkaTracesMessenger) getTopic(ctx context.Context, td ptrace.Traces) string {
	return getTopic[ptrace.ResourceSpans](ctx, e.config.Traces, e.config.TopicFromAttribute, td.ResourceSpans())
}

func (e *kafkaTracesMessenger) partitionData(td ptrace.Traces) iter.Seq2[[]byte, ptrace.Traces] {
	return func(yield func([]byte, ptrace.Traces) bool) {
		if e.config.PartitionTracesByID {
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
			return
		}
		if e.config.TopicFromAttribute != "" {
			newTraces := ptrace.NewTraces()
			target := newTraces.ResourceSpans().AppendEmpty()
			for _, resourceSpans := range td.ResourceSpans().All() {
				resourceSpans.CopyTo(target)
				// NOTE: The same ptrace.Traces instance (newTraces) is reused and mutated on each iteration.
				// Callers must treat the yielded pdata as ephemeral and must not retain it beyond
				// the current callback/iteration, as its contents will be overwritten on the next yield.
				if !yield(nil, newTraces) {
					return
				}
			}
			return
		}
		yield(nil, td)
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

func (e *kafkaLogsMessenger) marshalData(ld plog.Logs, yield func(key, value []byte)) error {
	return e.marshaler.MarshalLogs(ld, yield)
}

func (e *kafkaLogsMessenger) getTopic(ctx context.Context, ld plog.Logs) string {
	return getTopic[plog.ResourceLogs](ctx, e.config.Logs, e.config.TopicFromAttribute, ld.ResourceLogs())
}

func (e *kafkaLogsMessenger) partitionData(ld plog.Logs) iter.Seq2[[]byte, plog.Logs] {
	return func(yield func([]byte, plog.Logs) bool) {
		splitByResource := e.config.PartitionLogsByResourceAttributes ||
			(e.config.TopicFromAttribute != "" && !e.config.PartitionLogsByTraceID)
		if splitByResource {
			newLogs := plog.NewLogs()
			target := newLogs.ResourceLogs().AppendEmpty()
			for _, resourceLogs := range ld.ResourceLogs().All() {
				var key []byte
				if e.config.PartitionLogsByResourceAttributes {
					hash := pdatautil.MapHash(resourceLogs.Resource().Attributes())
					key = hash[:]
				}
				resourceLogs.CopyTo(target)
				// NOTE: The same plog.Logs instance (newLogs) is reused and mutated on each iteration.
				// Callers must treat the yielded pdata as ephemeral and must not retain it beyond
				// the current callback/iteration, as its contents will be overwritten on the next yield.
				if !yield(key, newLogs) {
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

func (e *kafkaMetricsMessenger) marshalData(md pmetric.Metrics, yield func(key, value []byte)) error {
	return e.marshaler.MarshalMetrics(md, yield)
}

func (e *kafkaMetricsMessenger) getTopic(ctx context.Context, md pmetric.Metrics) string {
	return getTopic[pmetric.ResourceMetrics](ctx, e.config.Metrics, e.config.TopicFromAttribute, md.ResourceMetrics())
}

func (e *kafkaMetricsMessenger) partitionData(md pmetric.Metrics) iter.Seq2[[]byte, pmetric.Metrics] {
	return func(yield func([]byte, pmetric.Metrics) bool) {
		splitByResource := e.config.PartitionMetricsByResourceAttributes ||
			e.config.TopicFromAttribute != ""
		if !splitByResource {
			yield(nil, md)
			return
		}
		newMetrics := pmetric.NewMetrics()
		target := newMetrics.ResourceMetrics().AppendEmpty()
		for _, resourceMetrics := range md.ResourceMetrics().All() {
			var key []byte
			if e.config.PartitionMetricsByResourceAttributes {
				hash := pdatautil.MapHash(resourceMetrics.Resource().Attributes())
				key = hash[:]
			}
			resourceMetrics.CopyTo(target)
			// NOTE: The same pmetric.Metrics instance (newMetrics) is reused and mutated on each iteration.
			// Callers must treat the yielded pdata as ephemeral and must not retain it beyond
			// the current callback/iteration, as its contents will be overwritten on the next yield.
			if !yield(key, newMetrics) {
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

func (e *kafkaProfilesMessenger) marshalData(ld pprofile.Profiles, yield func(key, value []byte)) error {
	return e.marshaler.MarshalProfiles(ld, yield)
}

func (e *kafkaProfilesMessenger) getTopic(ctx context.Context, ld pprofile.Profiles) string {
	return getTopic[pprofile.ResourceProfiles](ctx, e.config.Profiles, e.config.TopicFromAttribute, ld.ResourceProfiles())
}

func (e *kafkaProfilesMessenger) partitionData(pd pprofile.Profiles) iter.Seq2[[]byte, pprofile.Profiles] {
	return func(yield func([]byte, pprofile.Profiles) bool) {
		if e.config.TopicFromAttribute != "" {
			newProfiles := pprofile.NewProfiles()
			target := newProfiles.ResourceProfiles().AppendEmpty()
			for _, resourceProfiles := range pd.ResourceProfiles().All() {
				resourceProfiles.CopyTo(target)
				// NOTE: The same pprofile.Profiles instance (newProfiles) is reused and mutated on each iteration.
				// Callers must treat the yielded pdata as ephemeral and must not retain it beyond
				// the current callback/iteration, as its contents will be overwritten on the next yield.
				if !yield(nil, newProfiles) {
					return
				}
			}
			return
		}
		yield(nil, pd)
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
