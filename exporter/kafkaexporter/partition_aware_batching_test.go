// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkarequest"
)

func TestPartitionAwareBatchingConfig(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		require.False(t, cfg.PartitionAwareBatching.Enabled)
		require.Equal(t, defaultPartitionAwareTopicCacheSize, cfg.PartitionAwareBatching.TopicCacheSize)
	})

	t.Run("retains default capacity when enabled", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		conf := confmap.NewFromStringMap(map[string]any{
			"partition_aware_batching": map[string]any{"enabled": true},
		})
		require.NoError(t, conf.Unmarshal(cfg))
		require.True(t, cfg.PartitionAwareBatching.Enabled)
		require.Equal(t, defaultPartitionAwareTopicCacheSize, cfg.PartitionAwareBatching.TopicCacheSize)
	})

	t.Run("requires partition key", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionAwareBatching.Enabled = true
		require.ErrorIs(t, cfg.Validate(), errPartitionAwareRequiresKey)
	})

	t.Run("requires sticky key partitioner", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionAwareBatching.Enabled = true
		cfg.PartitionTracesByID = true
		cfg.RecordPartitioner = RecordPartitionerConfig{RoundRobin: &struct{}{}}
		require.ErrorIs(t, cfg.Validate(), errPartitionAwareRequiresSticky)
	})

	t.Run("requires positive topic cache capacity", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionAwareBatching.Enabled = true
		cfg.PartitionAwareBatching.TopicCacheSize = 0
		require.ErrorIs(t, cfg.Validate(), errPartitionAwareCacheSize)
	})

	t.Run("accepts supported configuration", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionAwareBatching.Enabled = true
		cfg.PartitionTracesByID = true
		require.NoError(t, cfg.Validate())
	})
}

func TestPartitionAwareProfilesUseLegacyConversion(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionAwareBatching.Enabled = true
	cfg.PartitionTracesByID = true
	require.NoError(t, cfg.Validate())

	profilesMarshaler, err := getProfilesMarshaler(cfg.Profiles.Encoding, componenttest.NewNopHost())
	require.NoError(t, err)
	exp := kafkaExporter[pprofile.Profiles]{
		cfg:       *cfg,
		messenger: &kafkaProfilesMessenger{config: *cfg, marshaler: profilesMarshaler},
		partitioner: newPartitionAwarePartitioner(
			kgo.StickyKeyPartitioner(nil),
			cfg.PartitionAwareBatching.TopicCacheSize,
		),
	}

	var records []kgo.Record
	err = exp.convertData(t.Context(), testdata.GenerateProfiles(2), func(record kgo.Record) {
		records = append(records, record)
	})
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, cfg.Profiles.Topic, records[0].Topic)
	require.NotEmpty(t, records[0].Value)
}

func TestPartitionAwareTracesAffinity(t *testing.T) {
	for _, hasher := range []string{HasherSaramaCompat, HasherMurmur2} {
		t.Run(hasher, func(t *testing.T) {
			const (
				topic          = "traces"
				partitionCount = 4
				traceCount     = 64
			)
			cfg := createDefaultConfig().(*Config)
			cfg.PartitionTracesByID = true
			cfg.Traces.Topic = topic
			cfg.RecordPartitioner.StickyKey.Hasher = hasher
			messenger := newTestTracesMessenger(t, *cfg)
			partitioner := newTestPartitionAwarePartitioner(t, *cfg, topic, partitionCount)
			topicPartitioner := partitioner.partitioner.ForTopic(topic)
			input := tracesWithUniqueIDs(traceCount)

			records, batched, err := collectPartitionAwareRecords(
				t.Context(), input, partitioner, messenger,
			)
			require.NoError(t, err)
			require.True(t, batched)
			require.NotEmpty(t, records)
			require.LessOrEqual(t, len(records), partitionCount)

			spans := 0
			for i := range records {
				planned, ok := kafkaclient.PlannedPartition(records[i].Context)
				require.True(t, ok)
				require.Equal(t, topic, records[i].Topic)
				require.Equal(t, topicPartitioner.Partition(&records[i], partitionCount), planned)

				traces, err := (&ptrace.ProtoUnmarshaler{}).UnmarshalTraces(records[i].Value)
				require.NoError(t, err)
				recordKeyPresent := false
				for _, resourceSpans := range traces.ResourceSpans().All() {
					for _, scopeSpans := range resourceSpans.ScopeSpans().All() {
						for _, span := range scopeSpans.Spans().All() {
							key := []byte(span.TraceID().String())
							require.Equal(t, topicPartitioner.Partition(&kgo.Record{Key: key}, partitionCount), planned)
							recordKeyPresent = recordKeyPresent || bytes.Equal(key, records[i].Key)
							spans++
						}
					}
				}
				require.True(t, recordKeyPresent)
			}
			require.Equal(t, traceCount, spans)
		})
	}
}

func TestPartitionAwareBatchingFallsBackUntilPartitionCountKnown(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionTracesByID = true
	messenger := newTestTracesMessenger(t, *cfg)
	partitioner := newPartitionAwarePartitioner(kgo.StickyKeyPartitioner(nil), defaultPartitionAwareTopicCacheSize)

	records, batched, err := collectPartitionAwareRecords(
		t.Context(), tracesWithUniqueIDs(2), partitioner, messenger,
	)
	require.NoError(t, err)
	require.False(t, batched)
	require.Empty(t, records)
}

func TestPartitionAwareBatchingFallsBackBeforeYielding(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionTracesByID = true
	cfg.TopicFromAttribute = "kafka.topic"
	messenger := newTestTracesMessenger(t, *cfg)
	partitioner := newTestPartitionAwarePartitioner(t, *cfg, "known-topic", 2)

	input := ptrace.NewTraces()
	for index, topic := range []string{"known-topic", "unknown-topic"} {
		resourceSpans := input.ResourceSpans().AppendEmpty()
		resourceSpans.Resource().Attributes().PutStr("kafka.topic", topic)
		resourceSpans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().
			SetTraceID(pcommon.TraceID{byte(index + 1)})
	}

	records, batched, err := collectPartitionAwareRecords(
		t.Context(), input, partitioner, messenger,
	)
	require.NoError(t, err)
	require.False(t, batched)
	require.Empty(t, records)
}

func TestPartitionAwareBatchingDynamicTopics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionTracesByID = true
	cfg.TopicFromAttribute = "kafka.topic"
	messenger := newTestTracesMessenger(t, *cfg)
	partitioner := newTestPartitionAwarePartitioner(t, *cfg, "topic-a", 2)
	partitioner.partitionCounts.Add("topic-b", 3)

	input := ptrace.NewTraces()
	for index, topic := range []string{"topic-a", "topic-b"} {
		resourceSpans := input.ResourceSpans().AppendEmpty()
		resourceSpans.Resource().Attributes().PutStr("kafka.topic", topic)
		span := resourceSpans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(pcommon.TraceID{byte(index + 1)})
	}

	records, batched, err := collectPartitionAwareRecords(
		t.Context(), input, partitioner, messenger,
	)
	require.NoError(t, err)
	require.True(t, batched)
	require.Len(t, records, 2)
	assert.ElementsMatch(t, []string{"topic-a", "topic-b"}, []string{records[0].Topic, records[1].Topic})
}

func TestPartitionAwareBatchingKeepsNilKeysUnplanned(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionLogsByTraceID = true
	marshaler, err := getLogsMarshaler(cfg.Logs.Encoding, componenttest.NewNopHost())
	require.NoError(t, err)
	messenger := &kafkaLogsMessenger{config: *cfg, marshaler: marshaler}
	partitioner := newTestPartitionAwarePartitioner(t, *cfg, cfg.Logs.Topic, 2)
	logs := plog.NewLogs()
	logRecords := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	logRecords.AppendEmpty()
	logRecords.AppendEmpty().SetTraceID(pcommon.TraceID{1})

	records, batched, err := collectPartitionAwareRecords(
		t.Context(), logs, partitioner, messenger,
	)
	require.NoError(t, err)
	require.True(t, batched)
	require.Len(t, records, 2)

	var planned, unplanned int
	for i := range records {
		if _, ok := kafkaclient.PlannedPartition(records[i].Context); ok {
			planned++
			require.NotNil(t, records[i].Key)
		} else {
			unplanned++
			require.Nil(t, records[i].Key)
		}
	}
	require.Equal(t, 1, planned)
	require.Equal(t, 1, unplanned)
}

func TestPartitionAwareBatchingResourceKeys(t *testing.T) {
	t.Run("logs", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionLogsByResourceAttributes = true
		marshaler, err := getLogsMarshaler(cfg.Logs.Encoding, componenttest.NewNopHost())
		require.NoError(t, err)
		messenger := &kafkaLogsMessenger{config: *cfg, marshaler: marshaler}
		partitioner := newTestPartitionAwarePartitioner(t, *cfg, cfg.Logs.Topic, 1)
		logs := plog.NewLogs()
		for _, service := range []string{"a", "b"} {
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			resourceLogs.Resource().Attributes().PutStr("service.name", service)
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		}

		records, batched, err := collectPartitionAwareRecords(
			t.Context(), logs, partitioner, messenger,
		)
		require.NoError(t, err)
		require.True(t, batched)
		require.Len(t, records, 1)
		output, err := (&plog.ProtoUnmarshaler{}).UnmarshalLogs(records[0].Value)
		require.NoError(t, err)
		require.Equal(t, 2, output.LogRecordCount())
	})

	t.Run("metrics", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.PartitionMetricsByResourceAttributes = true
		marshaler, err := getMetricsMarshaler(cfg.Metrics.Encoding, componenttest.NewNopHost())
		require.NoError(t, err)
		messenger := &kafkaMetricsMessenger{config: *cfg, marshaler: marshaler}
		partitioner := newTestPartitionAwarePartitioner(t, *cfg, cfg.Metrics.Topic, 1)
		metrics := pmetric.NewMetrics()
		for _, service := range []string{"a", "b"} {
			resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
			resourceMetrics.Resource().Attributes().PutStr("service.name", service)
			resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("requests")
			resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("errors")
		}

		records, batched, err := collectPartitionAwareRecords(
			t.Context(), metrics, partitioner, messenger,
		)
		require.NoError(t, err)
		require.True(t, batched)
		require.Len(t, records, 1)
		output, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(records[0].Value)
		require.NoError(t, err)
		require.Equal(t, 4, output.MetricCount())
		require.Equal(t, 2, output.ResourceMetrics().Len())
		for _, resourceMetrics := range output.ResourceMetrics().All() {
			require.Equal(t, 2, resourceMetrics.ScopeMetrics().Len())
		}
	})
}

func TestPartitionAwareBatchingFallsBackForPerItemMarshaler(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionLogsByTraceID = true
	cfg.Logs.Encoding = "raw"
	marshaler, err := getLogsMarshaler(cfg.Logs.Encoding, componenttest.NewNopHost())
	require.NoError(t, err)
	messenger := &kafkaLogsMessenger{config: *cfg, marshaler: marshaler}
	partitioner := newTestPartitionAwarePartitioner(t, *cfg, cfg.Logs.Topic, 1)
	logs := plog.NewLogs()
	logRecords := logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	var expectedKeys [][]byte
	for index := range 2 {
		record := logRecords.AppendEmpty()
		traceID := pcommon.TraceID{byte(index + 1)}
		record.SetTraceID(traceID)
		if index == 1 {
			record.Body().SetStr("message")
			expectedKeys = append(expectedKeys, []byte(traceID.String()))
		}
	}

	records, batched, err := collectPartitionAwareRecords(
		t.Context(), logs, partitioner, messenger,
	)
	require.NoError(t, err)
	require.False(t, batched)
	require.Empty(t, records)

	var actualKeys [][]byte
	for key, data := range messenger.partitionData(logs) {
		require.NoError(t, messenger.marshalData(data, func([]byte, []byte) {
			actualKeys = append(actualKeys, key)
		}))
	}
	assert.ElementsMatch(t, expectedKeys, actualKeys)
}

func TestPartitionAwareBatchingLeavesPartitionBatchIntact(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionTracesByID = true
	messenger := newTestTracesMessenger(t, *cfg)
	partitioner := newTestPartitionAwarePartitioner(t, *cfg, cfg.Traces.Topic, 1)

	input := ptrace.NewTraces()
	spans := input.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	for range 40 {
		span := spans.AppendEmpty()
		span.SetTraceID(pcommon.TraceID{1})
		span.Attributes().PutStr("payload", strings.Repeat("x", 100))
	}
	records, batched, err := collectPartitionAwareRecords(
		t.Context(), input, partitioner, messenger,
	)
	require.NoError(t, err)
	require.True(t, batched)
	require.Len(t, records, 1)
	require.Greater(t, len(records[0].Value), 2000)

	traces, err := (&ptrace.ProtoUnmarshaler{}).UnmarshalTraces(records[0].Value)
	require.NoError(t, err)
	require.Equal(t, input.SpanCount(), traces.SpanCount())
	planned, ok := kafkaclient.PlannedPartition(records[0].Context)
	require.True(t, ok)
	require.Zero(t, planned)
}

func TestPartitionAwarePartitionerHonorsPlannedPartitionAndRefreshesCount(t *testing.T) {
	const topic = "traces"
	partitioner := newPartitionAwarePartitioner(kgo.StickyKeyPartitioner(nil), defaultPartitionAwareTopicCacheSize)
	partitioner.partitionCounts.Add(topic, 2)
	key := []byte("trace-id")
	baseTopicPartitioner := partitioner.partitioner.ForTopic(topic)
	planned := baseTopicPartitioner.Partition(&kgo.Record{Key: key}, 2)

	topicPartitioner := partitioner.ForTopic(topic)
	record := &kgo.Record{
		Topic:   topic,
		Key:     key,
		Context: kafkaclient.WithPlannedPartition(t.Context(), planned),
	}
	require.True(t, topicPartitioner.RequiresConsistency(record))
	require.Equal(t, planned, topicPartitioner.Partition(record, 4))
	count, ok := partitioner.partitionCounts.Get(topic)
	require.True(t, ok)
	require.Equal(t, 4, count)
}

func TestPartitionAwarePartitionerCountTracking(t *testing.T) {
	const topic = "traces"
	key := []byte("trace-id")

	t.Run("does not learn from unkeyed records", func(t *testing.T) {
		partitioner := newPartitionAwarePartitioner(kgo.StickyKeyPartitioner(nil), defaultPartitionAwareTopicCacheSize)
		partitioner.ForTopic(topic).Partition(&kgo.Record{Topic: topic}, 3)
		_, ok := partitioner.partitionCounts.Get(topic)
		require.False(t, ok)
	})

	t.Run("learns from keyed records", func(t *testing.T) {
		partitioner := newPartitionAwarePartitioner(kgo.StickyKeyPartitioner(nil), defaultPartitionAwareTopicCacheSize)
		partitioner.ForTopic(topic).Partition(&kgo.Record{Topic: topic, Key: key}, 3)
		count, ok := partitioner.partitionCounts.Get(topic)
		require.True(t, ok)
		require.Equal(t, 3, count)
	})

	t.Run("invalid planned partition falls back to current hashing", func(t *testing.T) {
		partitioner := newPartitionAwarePartitioner(kgo.StickyKeyPartitioner(nil), defaultPartitionAwareTopicCacheSize)
		record := &kgo.Record{
			Topic:   topic,
			Key:     key,
			Context: kafkaclient.WithPlannedPartition(t.Context(), 3),
		}
		actual := partitioner.ForTopic(topic).Partition(record, 2)
		expected := partitioner.partitioner.ForTopic(topic).Partition(record, 2)
		require.Equal(t, expected, actual)
		count, ok := partitioner.partitionCounts.Get(topic)
		require.True(t, ok)
		require.Equal(t, 2, count)
	})
}

func TestTopicPartitionCountCacheEvictsLeastRecentlyUsed(t *testing.T) {
	cache := newPartitionAwarePartitioner(kgo.StickyKeyPartitioner(nil), 2).partitionCounts
	cache.Add("topic-a", 1)
	cache.Add("topic-b", 2)

	count, ok := cache.Get("topic-a")
	require.True(t, ok)
	require.Equal(t, 1, count)

	cache.Add("topic-c", 3)
	_, ok = cache.Get("topic-b")
	require.False(t, ok)
	count, ok = cache.Get("topic-a")
	require.True(t, ok)
	require.Equal(t, 1, count)
	count, ok = cache.Get("topic-c")
	require.True(t, ok)
	require.Equal(t, 3, count)
}

func TestPartitionAwareRequestPathFallbackThenBatch(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.PartitionAwareBatching.Enabled = true
	cfg.PartitionTracesByID = true
	exp, _ := newKgoMockTracesExporter(t, *cfg, componenttest.NewNopHost(), cfg.Traces.Topic)
	input := tracesWithUniqueIDs(2)
	converter := newRequestConverter(exp)
	pusher := newRequestPusher(exp)

	first, err := converter(t.Context(), input)
	require.NoError(t, err)
	require.Len(t, first.(*kafkarequest.Request).Records(), 2)
	require.NoError(t, pusher(t.Context(), first))

	second, err := converter(t.Context(), input)
	require.NoError(t, err)
	require.Len(t, second.(*kafkarequest.Request).Records(), 1)
	require.NoError(t, pusher(t.Context(), second))
}

func collectPartitionAwareRecords[T any](
	ctx context.Context,
	data T,
	partitioner *partitionAwarePartitioner,
	m messenger[T],
) ([]kgo.Record, bool, error) {
	var records []kgo.Record
	batched, err := m.partitionAwareRecords(ctx, data, partitioner, func(record kgo.Record) {
		records = append(records, record)
	})
	return records, batched, err
}

func newTestTracesMessenger(t *testing.T, cfg Config) *kafkaTracesMessenger {
	t.Helper()
	marshaler, err := getTracesMarshaler(cfg.Traces.Encoding, componenttest.NewNopHost())
	require.NoError(t, err)
	return &kafkaTracesMessenger{config: cfg, marshaler: marshaler}
}

func newTestPartitionAwarePartitioner(
	t *testing.T,
	cfg Config,
	topic string,
	partitionCount int,
) *partitionAwarePartitioner {
	t.Helper()
	partitioner, err := buildPartitioner(cfg.RecordPartitioner, componenttest.NewNopHost())
	require.NoError(t, err)
	result := newPartitionAwarePartitioner(partitioner, cfg.PartitionAwareBatching.TopicCacheSize)
	result.partitionCounts.Add(topic, partitionCount)
	return result
}

func tracesWithUniqueIDs(count int) ptrace.Traces {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	for index := range count {
		span := spans.AppendEmpty()
		span.SetTraceID(pcommon.TraceID{byte(index >> 8), byte(index + 1)})
	}
	return traces
}
