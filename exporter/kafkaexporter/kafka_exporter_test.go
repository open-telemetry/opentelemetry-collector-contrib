// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/kafkaclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestTracesPusher_attr_Kgo(t *testing.T) {
	config := createDefaultConfig().(*Config)
	attributeKey := "my_custom_topic_key_traces"
	expectedTopicFromAttribute := "topic_from_traces_attr_kgo"
	config.TopicFromAttribute = attributeKey

	exp, fakeCluster := newKgoMockTracesExporter(t, *config,
		componenttest.NewNopHost(), expectedTopicFromAttribute,
	)

	traces := testdata.GenerateTraces(1)
	traces.ResourceSpans().At(0).Resource().Attributes().PutStr(attributeKey, expectedTopicFromAttribute)

	err := exp.exportData(t.Context(), traces)
	require.NoError(t, err)

	records := fetchKgoRecords(t,
		fakeCluster.ListenAddrs(), expectedTopicFromAttribute, 1,
	)
	fakeCluster.Close()

	require.Len(t, records, 1, "expected one message to be produced, got %d", len(records))
	record := records[0]
	assert.Equal(t, expectedTopicFromAttribute, record.Topic, "message topic mismatch")

	assert.NotEmpty(t, record.Value)
	assert.Empty(t, record.Headers, "expected no headers for this test case")
	assert.Nil(t, record.Key, "expected nil key for this test case")
}

func TestTracesPusher_ctx_Kgo(t *testing.T) {
	t.Run("WithTopic", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		expectedTopicFromCtx := "my_kgo_topic_from_ctx"
		exp, fakeCluster := newKgoMockTracesExporter(t, *config,
			componenttest.NewNopHost(), expectedTopicFromCtx,
		)

		ctx := topic.WithTopic(t.Context(), expectedTopicFromCtx)
		traces := testdata.GenerateTraces(2)

		err := exp.exportData(ctx, traces)
		require.NoError(t, err)

		records := fetchKgoRecords(t,
			fakeCluster.ListenAddrs(), expectedTopicFromCtx, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, expectedTopicFromCtx, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.IncludeMetadataKeys = []string{"x-tenant-id", "x-request-ids"}
		exp, fakeCluster := newKgoMockTracesExporter(t, *config,
			componenttest.NewNopHost(), config.Traces.Topic,
		)

		defaultTopic := config.Traces.Topic // Fallback topic if not overridden
		ctx := client.NewContext(t.Context(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":   {"my_tenant_id"},
				"x-request-ids": {"987654321", "0187262"},
				"ignored-key":   {"some-value"}, // This should be ignored
			}),
		})
		traces := testdata.GenerateTraces(1)

		err := exp.exportData(ctx, traces)
		require.NoError(t, err)

		records := fetchKgoRecords(t,
			fakeCluster.ListenAddrs(), defaultTopic, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, defaultTopic, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)
		assert.ElementsMatch(t, []kgo.RecordHeader{
			{Key: "x-tenant-id", Value: []byte("my_tenant_id")},
			{Key: "x-request-ids", Value: []byte("987654321")},
			{Key: "x-request-ids", Value: []byte("0187262")},
		}, record.Headers, "message headers mismatch")
		assert.Nil(t, record.Key, "expected nil key for this test case")
	})
}

func TestTracesPusher_conf_err(t *testing.T) {
	t.Run("should return permanent err on marshal error", func(t *testing.T) {
		marshalErr := errors.New("marshal configuration error")
		host := extensionsHost{
			component.MustNewID("trace_encoding"): ptraceMarshalerFuncExtension(func(ptrace.Traces) ([]byte, error) {
				return nil, marshalErr
			}),
		}
		config := createDefaultConfig().(*Config)
		config.Traces.Encoding = "trace_encoding"
		exp, _ := newKgoMockTracesExporter(t, *config, host)

		err := exp.exportData(t.Context(), testdata.GenerateTraces(2))

		assert.True(t, consumererror.IsPermanent(err))
	})
}

func TestTracesPusher_partitioning(t *testing.T) {
	input := ptrace.NewTraces()
	resourceSpans := input.ResourceSpans().AppendEmpty()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	traceID1 := pcommon.TraceID{1}
	traceID2 := pcommon.TraceID{2}
	span1 := scopeSpans.Spans().AppendEmpty()
	span1.SetTraceID(traceID1)
	span2 := scopeSpans.Spans().AppendEmpty()
	span2.SetTraceID(traceID1)
	span3 := scopeSpans.Spans().AppendEmpty()
	span3.SetTraceID(traceID2)
	span4 := scopeSpans.Spans().AppendEmpty()
	span4.SetTraceID(traceID2)

	t.Run("default_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, fakeCluster := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), config.Traces.Topic)
		defer fakeCluster.Close()

		err := exp.exportData(t.Context(), input)
		require.NoError(t, err)

		records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), config.Traces.Topic, 1)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Nil(t, record.Key, "message key should be nil for default partitioning")
	})
	t.Run("jaeger_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.Traces.Encoding = "jaeger_json"
		exp, fakeCluster := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), config.Traces.Topic)
		defer fakeCluster.Close()

		err := exp.exportData(t.Context(), input)
		require.NoError(t, err)

		// Jaeger encodings produce one message per span,
		// and each one will have the trace ID as the key.
		records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), config.Traces.Topic, 4)
		require.Len(t, records, 4, "expected 4 messages (one per span) for Jaeger encoding")

		var keys [][]byte
		for _, record := range records {
			keys = append(keys, record.Key)
		}
		require.ElementsMatch(t, [][]byte{
			[]byte(traceID1.String()),
			[]byte(traceID1.String()),
			[]byte(traceID2.String()),
			[]byte(traceID2.String()),
		}, keys)
	})
	t.Run("trace_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.PartitionTracesByID = true
		exp, fakeCluster := newKgoMockTracesExporter(t, *config, componenttest.NewNopHost(), config.Traces.Topic)
		defer fakeCluster.Close()

		err := exp.exportData(t.Context(), input)
		require.NoError(t, err)

		// We should get one message per trace ID (2 messages total)
		records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), config.Traces.Topic, 2)
		require.Len(t, records, 2, "expected 2 messages (one per trace ID)")

		// Collect keys and traces
		var keys [][]byte
		var traces []ptrace.Traces
		for _, record := range records {
			keys = append(keys, record.Key)

			output, err := (&ptrace.ProtoUnmarshaler{}).UnmarshalTraces(record.Value)
			require.NoError(t, err)
			traces = append(traces, output)
		}

		expected := ptrace.NewTraces()
		scopeSpans1 := expected.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		span1.CopyTo(scopeSpans1.Spans().AppendEmpty())
		span2.CopyTo(scopeSpans1.Spans().AppendEmpty())
		scopeSpans2 := expected.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
		span3.CopyTo(scopeSpans2.Spans().AppendEmpty())
		span4.CopyTo(scopeSpans2.Spans().AppendEmpty())

		// Combine trace spans so we can compare ignoring order.
		require.Len(t, traces, 2)
		combined := traces[0]
		for _, rs := range traces[1].ResourceSpans().All() {
			rs.CopyTo(combined.ResourceSpans().AppendEmpty())
		}
		assert.NoError(t, ptracetest.CompareTraces(
			expected, combined,
			ptracetest.IgnoreResourceSpansOrder(),
			ptracetest.IgnoreScopeSpansOrder(),
			ptracetest.IgnoreSpansOrder(),
		))

		require.Len(t, keys, 2)
		require.ElementsMatch(t, [][]byte{
			[]byte(traceID1.String()),
			[]byte(traceID2.String()),
		}, keys)
	})
}

func TestTracesPusher_marshal_error(t *testing.T) {
	marshalErr := errors.New("failed to marshal")
	host := extensionsHost{
		component.MustNewID("trace_encoding"): ptraceMarshalerFuncExtension(func(ptrace.Traces) ([]byte, error) {
			return nil, marshalErr
		}),
	}
	config := createDefaultConfig().(*Config)
	config.Traces.Encoding = "trace_encoding"
	exp, _ := newKgoMockTracesExporter(t, *config, host)

	err := exp.exportData(t.Context(), testdata.GenerateTraces(2))
	assert.ErrorContains(t, err, marshalErr.Error())
}

func TestMetricsPusher_conf_err(t *testing.T) {
	t.Run("should return permanent err on marshal error", func(t *testing.T) {
		marshalErr := errors.New("marshal configuration error")
		host := extensionsHost{
			component.MustNewID("metric_encoding"): pmetricMarshalerFuncExtension(func(pmetric.Metrics) ([]byte, error) {
				return nil, marshalErr
			}),
		}
		config := createDefaultConfig().(*Config)
		config.Metrics.Encoding = "metric_encoding"
		exp, _ := newKgoMockMetricsExporter(t, *config, host)

		err := exp.exportData(t.Context(), testdata.GenerateMetrics(2))

		assert.True(t, consumererror.IsPermanent(err))
	})
}

func TestMetricsPusher_marshal_error(t *testing.T) {
	marshalErr := errors.New("failed to marshal")
	host := extensionsHost{
		component.MustNewID("metric_encoding"): pmetricMarshalerFuncExtension(func(pmetric.Metrics) ([]byte, error) {
			return nil, marshalErr
		}),
	}
	config := createDefaultConfig().(*Config)
	config.Metrics.Encoding = "metric_encoding"
	exp, _ := newKgoMockMetricsExporter(t, *config, host)

	err := exp.exportData(t.Context(), testdata.GenerateMetrics(2))
	assert.ErrorContains(t, err, marshalErr.Error())
}

func TestMetricsDataPusher_Kgo(t *testing.T) {
	config := createDefaultConfig().(*Config)

	exp, fakeCluster := newKgoMockMetricsExporter(t, *config,
		componenttest.NewNopHost(), config.Metrics.Topic,
	)

	metrics := testdata.GenerateMetrics(2)
	err := exp.exportData(t.Context(), metrics)
	require.NoError(t, err)

	expectedTopic := config.Metrics.Topic

	records := fetchKgoRecords(t,
		fakeCluster.ListenAddrs(), expectedTopic, 1,
	)
	fakeCluster.Close()

	require.Len(t, records, 1, "expected one message to be produced for metrics batch")
	record := records[0]
	assert.Equal(t, expectedTopic, record.Topic, "message topic mismatch")

	assert.NotEmpty(t, record.Value)

	assert.Empty(t, record.Headers, "expected no headers for default config")
	assert.Nil(t, record.Key, "expected nil key for default config")
}

func TestMetricsDataPusher_attr_Kgo(t *testing.T) {
	config := createDefaultConfig().(*Config)
	attributeKey := "my_custom_topic_key_metrics"
	expectedTopicFromAttribute := "topic_from_metrics_attr_kgo"
	config.TopicFromAttribute = attributeKey // This applies to all signals if not overridden per signal
	// For metrics specifically, it would be config.Metrics.TopicFromAttribute if that existed,
	// but TopicFromAttribute is a top-level config in the current Config struct for this exporter.

	exp, fakeCluster := newKgoMockMetricsExporter(t, *config,
		componenttest.NewNopHost(), expectedTopicFromAttribute,
	)

	metrics := testdata.GenerateMetrics(1)
	// Add the attribute to the first resource's attributes
	metrics.ResourceMetrics().At(0).Resource().Attributes().PutStr(attributeKey, expectedTopicFromAttribute)

	err := exp.exportData(t.Context(), metrics)
	require.NoError(t, err)

	consumerSeedBrokers := fakeCluster.ListenAddrs()
	records := fetchKgoRecords(t,
		consumerSeedBrokers, expectedTopicFromAttribute, 1,
	)

	require.Len(t, records, 1, "expected one message to be produced")
	record := records[0]
	assert.Equal(t, expectedTopicFromAttribute, record.Topic, "message should be sent to topic from attribute")

	assert.NotEmpty(t, record.Value)
	assert.Empty(t, record.Headers, "expected no headers for this test case")
	assert.Nil(t, record.Key, "expected nil key for this test case")
}

func TestMetricsDataPusher_ctx_Kgo(t *testing.T) {
	t.Run("WithTopic", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		expectedTopicFromCtx := "my_kgo_metrics_topic_from_ctx"
		exp, fakeCluster := newKgoMockMetricsExporter(t, *config,
			componenttest.NewNopHost(), expectedTopicFromCtx,
		)

		ctx := topic.WithTopic(t.Context(), expectedTopicFromCtx)
		metrics := testdata.GenerateMetrics(2)

		err := exp.exportData(ctx, metrics)
		require.NoError(t, err)

		consumerSeedBrokers := fakeCluster.ListenAddrs()
		records := fetchKgoRecords(t,
			consumerSeedBrokers, expectedTopicFromCtx, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, expectedTopicFromCtx, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.IncludeMetadataKeys = []string{"x-metrics-tenant-id", "x-metrics-req-id"}
		exp, fakeCluster := newKgoMockMetricsExporter(t, *config,
			componenttest.NewNopHost(), config.Metrics.Topic,
		)

		ctx := client.NewContext(t.Context(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-metrics-tenant-id": {"metrics_tenant"},
				"x-metrics-req-id":    {"req123", "req456"},
				"ignored-key":         {"some-value"},
			}),
		})
		metrics := testdata.GenerateMetrics(1)

		err := exp.exportData(ctx, metrics)
		require.NoError(t, err)

		consumerSeedBrokers := fakeCluster.ListenAddrs()
		records := fetchKgoRecords(t,
			consumerSeedBrokers, config.Metrics.Topic, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, config.Metrics.Topic, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)

		expectedHeaders := []kgo.RecordHeader{
			{Key: "x-metrics-tenant-id", Value: []byte("metrics_tenant")},
			{Key: "x-metrics-req-id", Value: []byte("req123")},
			{Key: "x-metrics-req-id", Value: []byte("req456")},
		}
		assert.ElementsMatch(t, expectedHeaders, record.Headers, "message headers mismatch")
	})
}

func TestLogsDataPusher_attr_Kgo(t *testing.T) {
	config := createDefaultConfig().(*Config)
	attributeKey := "my_custom_topic_key_logs"
	expectedTopicFromAttribute := "topic_from_logs_attr_kgo"
	config.TopicFromAttribute = attributeKey

	exp, fakeCluster := newKgoMockLogsExporter(t, *config,
		componenttest.NewNopHost(), expectedTopicFromAttribute,
	)

	logs := testdata.GenerateLogs(1)
	logs.ResourceLogs().At(0).Resource().Attributes().PutStr(attributeKey, expectedTopicFromAttribute)

	err := exp.exportData(t.Context(), logs)
	require.NoError(t, err)

	records := fetchKgoRecords(t,
		fakeCluster.ListenAddrs(), expectedTopicFromAttribute, 1,
	)
	fakeCluster.Close()

	require.Len(t, records, 1, "expected one message to be produced, got %d", len(records))
	record := records[0]
	assert.Equal(t, expectedTopicFromAttribute, record.Topic, "message topic mismatch")

	assert.NotEmpty(t, record.Value)
	assert.Empty(t, record.Headers, "expected no headers for this test case")
	assert.Nil(t, record.Key, "expected nil key for this test case")
}

func TestLogsDataPusher_ctx_Kgo(t *testing.T) {
	t.Run("WithTopic", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		expectedTopicFromCtx := "my_kgo_logs_topic_from_ctx"
		exp, fakeCluster := newKgoMockLogsExporter(t, *config,
			componenttest.NewNopHost(), expectedTopicFromCtx,
		)

		ctx := topic.WithTopic(t.Context(), expectedTopicFromCtx)
		logs := testdata.GenerateLogs(2)

		err := exp.exportData(ctx, logs)
		require.NoError(t, err)

		records := fetchKgoRecords(t,
			fakeCluster.ListenAddrs(), expectedTopicFromCtx, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, expectedTopicFromCtx, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.IncludeMetadataKeys = []string{"x-tenant-id", "x-request-ids"}
		exp, fakeCluster := newKgoMockLogsExporter(t, *config,
			componenttest.NewNopHost(), config.Logs.Topic,
		)

		defaultTopic := config.Logs.Topic // Fallback topic if not overridden
		ctx := client.NewContext(t.Context(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":   {"my_tenant_id"},
				"x-request-ids": {"987654321", "0187262"},
				"ignored-key":   {"some-value"}, // This should be ignored
			}),
		})
		logs := testdata.GenerateLogs(1)

		err := exp.exportData(ctx, logs)
		require.NoError(t, err)

		records := fetchKgoRecords(t,
			fakeCluster.ListenAddrs(), defaultTopic, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, defaultTopic, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)
		expectedHeaders := []kgo.RecordHeader{
			{Key: "x-tenant-id", Value: []byte("my_tenant_id")},
			{Key: "x-request-ids", Value: []byte("987654321")},
			{Key: "x-request-ids", Value: []byte("0187262")},
		}
		assert.ElementsMatch(t, expectedHeaders, record.Headers, "message headers mismatch")
	})

	// Produce message that exceeds MaxMessageBytes to trigger a permanent non-retriable error.
	t.Run("WithNonRetriableError", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.Producer.MaxMessageBytes = 512
		exp, fakeCluster := newKgoMockLogsExporter(t, *config,
			componenttest.NewNopHost(), config.Logs.Topic,
		)
		defer fakeCluster.Close()

		// ensure we have a big payload to trigger the error
		logs := testdata.GenerateLogs(20)

		err := exp.exportData(t.Context(), logs)
		require.ErrorAs(t, err, &kerr.MessageTooLarge, "expected MessageTooLarge error")
		assert.True(t, consumererror.IsPermanent(err), "expected permanent error")
	})

	// Produce message to an unknown topic to trigger a retriable error.
	t.Run("WithRetriableError", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, fakeCluster := newKgoMockLogsExporter(t, *config,
			componenttest.NewNopHost(), "non_existing_topic",
		)
		defer fakeCluster.Close()

		logs := testdata.GenerateLogs(1)

		err := exp.exportData(t.Context(), logs)
		require.ErrorAs(t, err, &kerr.UnknownTopicOrPartition, "expected UnknownTopicOrPartition error")
		assert.False(t, consumererror.IsPermanent(err), "expected retriable error")
	})
}

func TestLogsPusher_conf_err(t *testing.T) {
	t.Run("should return permanent err on marshal error", func(t *testing.T) {
		marshalErr := errors.New("marshal configuration error")
		host := extensionsHost{
			component.MustNewID("log_encoding"): plogMarshalerFuncExtension(func(plog.Logs) ([]byte, error) {
				return nil, marshalErr
			}),
		}
		config := createDefaultConfig().(*Config)
		config.Logs.Encoding = "log_encoding"
		exp, _ := newKgoMockLogsExporter(t, *config, host)

		err := exp.exportData(t.Context(), testdata.GenerateLogs(2))

		assert.True(t, consumererror.IsPermanent(err))
	})
}

func TestLogsPusher_marshal_error(t *testing.T) {
	marshalErr := errors.New("failed to marshal")
	host := extensionsHost{
		component.MustNewID("log_encoding"): plogMarshalerFuncExtension(func(plog.Logs) ([]byte, error) {
			return nil, marshalErr
		}),
	}
	config := createDefaultConfig().(*Config)
	config.Logs.Encoding = "log_encoding"
	exp, _ := newKgoMockLogsExporter(t, *config, host)

	err := exp.exportData(t.Context(), testdata.GenerateLogs(2))
	assert.ErrorContains(t, err, marshalErr.Error())
}

func TestProfilesPusher_attr_Kgo(t *testing.T) {
	config := createDefaultConfig().(*Config)
	attributeKey := "my_custom_topic_key_profile"
	expectedTopicFromAttribute := "topic_from_profiles_attr_kgo"
	config.TopicFromAttribute = attributeKey

	exp, fakeCluster := newKgoMockProfilesExporter(t, *config,
		componenttest.NewNopHost(), expectedTopicFromAttribute,
	)

	profiles := testdata.GenerateProfiles(1)
	profiles.ResourceProfiles().At(0).Resource().Attributes().PutStr(attributeKey, expectedTopicFromAttribute)

	err := exp.exportData(t.Context(), profiles)
	require.NoError(t, err)

	records := fetchKgoRecords(t,
		fakeCluster.ListenAddrs(), expectedTopicFromAttribute, 1,
	)
	fakeCluster.Close()

	require.Len(t, records, 1, "expected one message to be produced, got %d", len(records))
	record := records[0]
	assert.Equal(t, expectedTopicFromAttribute, record.Topic, "message topic mismatch")

	assert.NotEmpty(t, record.Value)
	assert.Empty(t, record.Headers, "expected no headers for this test case")
	assert.Nil(t, record.Key, "expected nil key for this test case")
}

func TestProfilesPusher_ctx_Kgo(t *testing.T) {
	t.Run("WithTopic", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		expectedTopicFromCtx := "my_kgo_topic_from_ctx"
		exp, fakeCluster := newKgoMockProfilesExporter(t, *config,
			componenttest.NewNopHost(), expectedTopicFromCtx,
		)

		ctx := topic.WithTopic(t.Context(), expectedTopicFromCtx)
		profiles := testdata.GenerateProfiles(2)

		err := exp.exportData(ctx, profiles)
		require.NoError(t, err)

		records := fetchKgoRecords(t,
			fakeCluster.ListenAddrs(), expectedTopicFromCtx, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, expectedTopicFromCtx, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)
	})

	t.Run("WithMetadata", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.IncludeMetadataKeys = []string{"x-tenant-id", "x-request-ids"}
		exp, fakeCluster := newKgoMockProfilesExporter(t, *config,
			componenttest.NewNopHost(), config.Profiles.Topic,
		)

		defaultTopic := config.Profiles.Topic // Fallback topic if not overridden
		ctx := client.NewContext(t.Context(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":   {"my_tenant_id"},
				"x-request-ids": {"987654321", "0187262"},
				"ignored-key":   {"some-value"}, // This should be ignored
			}),
		})
		profiles := testdata.GenerateProfiles(1)

		err := exp.exportData(ctx, profiles)
		require.NoError(t, err)

		records := fetchKgoRecords(t,
			fakeCluster.ListenAddrs(), defaultTopic, 1,
		)
		require.Len(t, records, 1, "expected one message to be produced")
		record := records[0]
		assert.Equal(t, defaultTopic, record.Topic, "message topic mismatch")
		assert.NotEmpty(t, record.Value)
		assert.ElementsMatch(t, []kgo.RecordHeader{
			{Key: "x-tenant-id", Value: []byte("my_tenant_id")},
			{Key: "x-request-ids", Value: []byte("987654321")},
			{Key: "x-request-ids", Value: []byte("0187262")},
		}, record.Headers, "message headers mismatch")
		assert.Nil(t, record.Key, "expected nil key for this test case")
	})
}

func TestProfilesPusher_conf_err(t *testing.T) {
	t.Run("should return permanent err on marshal error", func(t *testing.T) {
		marshalErr := errors.New("marshal configuration error")
		host := extensionsHost{
			component.MustNewID("profile_encoding"): pprofileMarshalerFuncExtension(func(pprofile.Profiles) ([]byte, error) {
				return nil, marshalErr
			}),
		}
		config := createDefaultConfig().(*Config)
		config.Profiles.Encoding = "profile_encoding"
		exp, _ := newKgoMockProfilesExporter(t, *config, host)

		err := exp.exportData(t.Context(), testdata.GenerateProfiles(2))

		assert.True(t, consumererror.IsPermanent(err))
	})
}

func TestProfilesPusher_marshal_error(t *testing.T) {
	marshalErr := errors.New("failed to marshal")
	host := extensionsHost{
		component.MustNewID("profile_encoding"): pprofileMarshalerFuncExtension(func(pprofile.Profiles) ([]byte, error) {
			return nil, marshalErr
		}),
	}
	config := createDefaultConfig().(*Config)
	config.Profiles.Encoding = "profile_encoding"
	exp, _ := newKgoMockProfilesExporter(t, *config, host)

	err := exp.exportData(t.Context(), testdata.GenerateProfiles(2))
	assert.ErrorContains(t, err, marshalErr.Error())
}

func Test_GetTopic(t *testing.T) {
	tests := []struct {
		name               string
		topicFromAttribute string
		signalCfg          SignalConfig
		ctx                context.Context
		resource           any
		wantTopic          string
	}{
		// topicFromAttribute tests.
		{
			name:               "Valid metric attribute, return topic name",
			topicFromAttribute: "resource-attr",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(t.Context(), "context-topic"),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "resource-attr-val-1",
		},
		{
			name:               "Valid trace attribute, return topic name",
			topicFromAttribute: "resource-attr",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(t.Context(), "context-topic"),
			resource:           testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic:          "resource-attr-val-1",
		},
		{
			name:               "Valid log attribute, return topic name",
			topicFromAttribute: "resource-attr",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(t.Context(), "context-topic"),
			resource:           testdata.GenerateLogs(1).ResourceLogs(),
			wantTopic:          "resource-attr-val-1",
		},
		{
			name:               "Attribute not found",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                t.Context(),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "defaultTopic",
		},
		// Nonexistent attribute tests.
		{
			name:               "Valid metric context, return topic name",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(t.Context(), "context-topic"),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "context-topic",
		},
		{
			name:               "Valid trace context, return topic name",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(t.Context(), "context-topic"),
			resource:           testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic:          "context-topic",
		},
		{
			name:               "Valid log context, return topic name",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(t.Context(), "context-topic"),
			resource:           testdata.GenerateLogs(1).ResourceLogs(),
			wantTopic:          "context-topic",
		},
		// Generic known failure modes.
		{
			name:               "Attribute not found",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                t.Context(),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "defaultTopic",
		},
		{
			name:      "TopicFromAttribute, return default topic",
			ctx:       t.Context(),
			signalCfg: SignalConfig{Topic: "defaultTopic"},
			resource:  testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic: "defaultTopic",
		},
		// topicFromMetadata tests.
		{
			name: "Metrics topic from metadata",
			signalCfg: SignalConfig{
				Topic:                "defaultTopic",
				TopicFromMetadataKey: "metrics_topic_metadata",
			},
			ctx: client.NewContext(t.Context(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"metrics_topic_metadata": {"my_metrics_topic"},
				})},
			),
			resource:  testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic: "my_metrics_topic",
		},
		{
			name: "Logs topic from metadata",
			signalCfg: SignalConfig{
				Topic:                "defaultTopic",
				TopicFromMetadataKey: "logs_topic_metadata",
			},
			ctx: client.NewContext(t.Context(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"logs_topic_metadata": {"my_logs_topic"},
				})},
			),
			resource:  testdata.GenerateLogs(1).ResourceLogs(),
			wantTopic: "my_logs_topic",
		},
		{
			name: "Traces topic from metadata",
			signalCfg: SignalConfig{
				Topic:                "defaultTopic",
				TopicFromMetadataKey: "traces_topic_metadata",
			},
			ctx: client.NewContext(t.Context(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"traces_topic_metadata": {"my_traces_topic"},
				})},
			),
			resource:  testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic: "my_traces_topic",
		},
		{
			name: "metadata key not found uses default topic",
			signalCfg: SignalConfig{
				Topic:                "defaultTopic",
				TopicFromMetadataKey: "key not found",
			},
			ctx: client.NewContext(t.Context(),
				client.Info{Metadata: client.NewMetadata(map[string][]string{
					"traces_topic_metadata": {"my_traces_topic"},
				})},
			),
			resource:  testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic: "defaultTopic",
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			topic := ""
			switch r := tests[i].resource.(type) {
			case pmetric.ResourceMetricsSlice:
				topic = getTopic[pmetric.ResourceMetrics](tests[i].ctx, tests[i].signalCfg, tests[i].topicFromAttribute, r)
			case ptrace.ResourceSpansSlice:
				topic = getTopic[ptrace.ResourceSpans](tests[i].ctx, tests[i].signalCfg, tests[i].topicFromAttribute, r)
			case plog.ResourceLogsSlice:
				topic = getTopic[plog.ResourceLogs](tests[i].ctx, tests[i].signalCfg, tests[i].topicFromAttribute, r)
			}
			assert.Equal(t, tests[i].wantTopic, topic)
		})
	}
}

type extensionsHost map[component.ID]component.Component

func (m extensionsHost) GetExtensions() map[component.ID]component.Component {
	return m
}

type ptraceMarshalerFuncExtension func(ptrace.Traces) ([]byte, error)

func (f ptraceMarshalerFuncExtension) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	return f(td)
}

func (ptraceMarshalerFuncExtension) Start(context.Context, component.Host) error {
	return nil
}

func (ptraceMarshalerFuncExtension) Shutdown(context.Context) error {
	return nil
}

type pmetricMarshalerFuncExtension func(pmetric.Metrics) ([]byte, error)

func (f pmetricMarshalerFuncExtension) MarshalMetrics(td pmetric.Metrics) ([]byte, error) {
	return f(td)
}

func (pmetricMarshalerFuncExtension) Start(context.Context, component.Host) error {
	return nil
}

func (pmetricMarshalerFuncExtension) Shutdown(context.Context) error {
	return nil
}

type plogMarshalerFuncExtension func(plog.Logs) ([]byte, error)

func (f plogMarshalerFuncExtension) MarshalLogs(td plog.Logs) ([]byte, error) {
	return f(td)
}

func (plogMarshalerFuncExtension) Start(context.Context, component.Host) error {
	return nil
}

func (plogMarshalerFuncExtension) Shutdown(context.Context) error {
	return nil
}

type pprofileMarshalerFuncExtension func(pprofile.Profiles) ([]byte, error)

func (f pprofileMarshalerFuncExtension) MarshalProfiles(td pprofile.Profiles) ([]byte, error) {
	return f(td)
}

func (pprofileMarshalerFuncExtension) Start(context.Context, component.Host) error {
	return nil
}

func (pprofileMarshalerFuncExtension) Shutdown(context.Context) error {
	return nil
}

func newKgoMockLogsExporter(t *testing.T, cfg Config, host component.Host, topics ...string) (*kafkaExporter[plog.Logs], *kfake.Cluster) {
	exp := newLogsExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	cluster := configureExporter(t, exp, cfg, host, topics...)
	return exp, cluster
}

func newKgoMockTracesExporter(t *testing.T, cfg Config, host component.Host, topics ...string) (*kafkaExporter[ptrace.Traces], *kfake.Cluster) {
	exp := newTracesExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	cluster := configureExporter(t, exp, cfg, host, topics...)
	return exp, cluster
}

func newKgoMockMetricsExporter(t *testing.T, cfg Config, host component.Host, topics ...string) (*kafkaExporter[pmetric.Metrics], *kfake.Cluster) {
	exp := newMetricsExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	cluster := configureExporter(t, exp, cfg, host, topics...)
	return exp, cluster
}

func newKgoMockProfilesExporter(t *testing.T, cfg Config, host component.Host, topics ...string) (*kafkaExporter[pprofile.Profiles], *kfake.Cluster) {
	exp := newProfilesExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	cluster := configureExporter(t, exp, cfg, host, topics...)
	return exp, cluster
}

func configureExporter[T any](tb testing.TB,
	exp *kafkaExporter[T], cfg Config, host component.Host, topics ...string,
) *kfake.Cluster {
	cluster, kcfg := kafkatest.NewCluster(tb, kfake.SeedTopics(1, topics...))

	// Create a kgo.Client using the broker addresses from the fake cluster.
	kgoClientOpts := []kgo.Opt{
		kgo.SeedBrokers(kcfg.Brokers...),
		kgo.ClientID(cfg.ClientID),
	}

	client, err := kafka.NewFranzSyncProducer(tb.Context(), host, kcfg,
		cfg.Producer, 1*time.Second, zap.NewNop(), kgoClientOpts...)
	require.NoError(tb, err, "failed to create kgo.Client with fake cluster addresses")

	messenger, err := exp.newMessenger(host) // messenger implements Marshaler[pmetric.Metrics]
	require.NoError(tb, err, "failed to create messenger for metrics")

	exp.messenger = messenger
	exp.producer = kafkaclient.NewFranzSyncProducer(client, cfg.IncludeMetadataKeys)

	tb.Cleanup(func() { assert.NoError(tb, exp.Close(tb.Context())) })
	return cluster
}

// fetchKgoRecords polls a franz-go topic and returns records produced to that topic.
// maxRecords specifies the maximum number of records to fetch.
func fetchKgoRecords(tb testing.TB, brokers []string, topic string, maxRecords int) []*kgo.Record {
	clientOpts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup("group-id" + topic),
	}
	consumerClient, err := kgo.NewClient(clientOpts...)
	require.NoError(tb, err, "failed to create kgo consumer client")
	defer consumerClient.Close()

	var records []*kgo.Record
	fetches := consumerClient.PollRecords(tb.Context(), maxRecords)
	require.NoError(tb, fetches.Err(), "error polling records")
	fetches.EachRecord(func(r *kgo.Record) {
		records = append(records, r)
	})
	return records
}
