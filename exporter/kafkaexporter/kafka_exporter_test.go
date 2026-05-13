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
	"go.opentelemetry.io/collector/component/componentstatus"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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

func TestLogsPusher_partitioning(t *testing.T) {
	// Build input with 2 distinct resources, each with 1 scope + 1 log record.
	input := plog.NewLogs()

	rl1 := input.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", "svc-a")
	lr1 := rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr1.Body().SetStr("log from svc-a")

	rl2 := input.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("service.name", "svc-b")
	lr2 := rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	lr2.Body().SetStr("log from svc-b")

	t.Run("default_no_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, fakeCluster := newKgoMockLogsExporter(t, *config,
			componenttest.NewNopHost(), config.Logs.Topic)
		defer fakeCluster.Close()

		err := exp.exportData(t.Context(), input)
		require.NoError(t, err)

		records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), config.Logs.Topic, 1)
		require.Len(t, records, 1, "expected one message (no partitioning)")
		assert.Nil(t, records[0].Key, "key should be nil for default partitioning")
	})

	t.Run("resource_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.PartitionLogsByResourceAttributes = true
		exp, fakeCluster := newKgoMockLogsExporter(t, *config,
			componenttest.NewNopHost(), config.Logs.Topic)
		defer fakeCluster.Close()

		err := exp.exportData(t.Context(), input)
		require.NoError(t, err)

		// 2 resources -> 2 messages.
		records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), config.Logs.Topic, 2)
		require.Len(t, records, 2, "expected 2 messages (one per resource)")

		// Each record must have a non-nil key (the resource attribute hash).
		var keys [][]byte
		var allLogs []plog.Logs
		for _, record := range records {
			assert.NotNil(t, record.Key, "partition key must not be nil")
			keys = append(keys, record.Key)

			output, err := (&plog.ProtoUnmarshaler{}).UnmarshalLogs(record.Value)
			require.NoError(t, err)
			allLogs = append(allLogs, output)
		}

		// Keys must be distinct (different resource attributes -> different hash).
		assert.NotEqual(t, keys[0], keys[1], "keys for distinct resources must differ")

		// Combine deserialized logs for content comparison.
		combined := allLogs[0]
		for _, l := range allLogs[1:] {
			for _, rl := range l.ResourceLogs().All() {
				rl.CopyTo(combined.ResourceLogs().AppendEmpty())
			}
		}
		assert.NoError(t, plogtest.CompareLogs(
			input, combined,
			plogtest.IgnoreResourceLogsOrder(),
			plogtest.IgnoreScopeLogsOrder(),
			plogtest.IgnoreLogRecordsOrder(),
		))
	})
}

func TestMetricsPusher_partitioning(t *testing.T) {
	// Build input with 3 distinct resources, each with 1 gauge metric.
	input := pmetric.NewMetrics()

	rm1 := input.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("host.name", "host-1")
	m1 := rm1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m1.SetName("cpu.utilization")
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)

	rm2 := input.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("host.name", "host-2")
	m2 := rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m2.SetName("cpu.utilization")
	m2.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(84)

	rm3 := input.ResourceMetrics().AppendEmpty()
	rm3.Resource().Attributes().PutStr("host.name", "host-3")
	m3 := rm3.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m3.SetName("cpu.utilization")
	m3.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(13)

	t.Run("default_no_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, fakeCluster := newKgoMockMetricsExporter(t, *config,
			componenttest.NewNopHost(), config.Metrics.Topic)
		defer fakeCluster.Close()

		err := exp.exportData(t.Context(), input)
		require.NoError(t, err)

		records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), config.Metrics.Topic, 1)
		require.Len(t, records, 1, "expected one message (no partitioning)")
		assert.Nil(t, records[0].Key, "key should be nil for default partitioning")
	})

	t.Run("resource_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.PartitionMetricsByResourceAttributes = true
		exp, fakeCluster := newKgoMockMetricsExporter(t, *config,
			componenttest.NewNopHost(), config.Metrics.Topic)
		defer fakeCluster.Close()

		err := exp.exportData(t.Context(), input)
		require.NoError(t, err)

		// 3 resources -> 3 messages.
		records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), config.Metrics.Topic, 3)
		require.Len(t, records, 3, "expected 3 messages (one per resource)")

		// Each record must have a non-nil key.
		keySet := make(map[string]struct{})
		var allMetrics []pmetric.Metrics
		for _, record := range records {
			assert.NotNil(t, record.Key, "partition key must not be nil")
			keySet[string(record.Key)] = struct{}{}

			output, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(record.Value)
			require.NoError(t, err)
			allMetrics = append(allMetrics, output)
		}

		// All 3 keys must be distinct.
		assert.Len(t, keySet, 3, "keys for distinct resources must all differ")

		// Combine deserialized metrics for content comparison.
		combined := allMetrics[0]
		for _, md := range allMetrics[1:] {
			for _, rm := range md.ResourceMetrics().All() {
				rm.CopyTo(combined.ResourceMetrics().AppendEmpty())
			}
		}
		assert.NoError(t, pmetrictest.CompareMetrics(
			input, combined,
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
		))
	})
}

func TestPartitionData_TopicFromAttributeSplitsMetrics(t *testing.T) {
	cfg := Config{TopicFromAttribute: "k8s.namespace.name"}
	e := &kafkaMetricsMessenger{config: cfg}

	md := pmetric.NewMetrics()
	r1 := md.ResourceMetrics().AppendEmpty()
	r1.Resource().Attributes().PutStr("k8s.namespace.name", "ns-alpha")
	r1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	r2 := md.ResourceMetrics().AppendEmpty()
	r2.Resource().Attributes().PutStr("k8s.namespace.name", "ns-beta")
	r2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()

	var chunks []pmetric.Metrics
	var keys [][]byte
	for key, data := range e.partitionData(md) {
		clone := pmetric.NewMetrics()
		data.CopyTo(clone)
		chunks = append(chunks, clone)
		keys = append(keys, key)
	}

	require.Len(t, chunks, 2, "should yield one chunk per resource")
	require.Nil(t, keys[0], "key should be nil (no explicit partitioning)")
	require.Nil(t, keys[1], "key should be nil (no explicit partitioning)")

	// Each chunk should have exactly one ResourceMetrics.
	require.Equal(t, 1, chunks[0].ResourceMetrics().Len())
	require.Equal(t, 1, chunks[1].ResourceMetrics().Len())

	// Verify each chunk carries the correct resource attribute.
	v0, _ := chunks[0].ResourceMetrics().At(0).Resource().Attributes().Get("k8s.namespace.name")
	v1, _ := chunks[1].ResourceMetrics().At(0).Resource().Attributes().Get("k8s.namespace.name")
	require.Equal(t, "ns-alpha", v0.Str())
	require.Equal(t, "ns-beta", v1.Str())
}

func TestPartitionData_TopicFromAttributeSplitsLogs(t *testing.T) {
	cfg := Config{TopicFromAttribute: "k8s.namespace.name"}
	e := &kafkaLogsMessenger{config: cfg}

	ld := plog.NewLogs()
	r1 := ld.ResourceLogs().AppendEmpty()
	r1.Resource().Attributes().PutStr("k8s.namespace.name", "ns-alpha")
	r1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	r2 := ld.ResourceLogs().AppendEmpty()
	r2.Resource().Attributes().PutStr("k8s.namespace.name", "ns-beta")
	r2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	var count int
	for range e.partitionData(ld) {
		count++
	}
	require.Equal(t, 2, count, "should yield one chunk per resource")
}

func TestPartitionData_TopicFromAttributeSplitsTraces(t *testing.T) {
	cfg := Config{TopicFromAttribute: "k8s.namespace.name"}
	e := &kafkaTracesMessenger{config: cfg}

	td := ptrace.NewTraces()
	r1 := td.ResourceSpans().AppendEmpty()
	r1.Resource().Attributes().PutStr("k8s.namespace.name", "ns-alpha")
	r1.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	r2 := td.ResourceSpans().AppendEmpty()
	r2.Resource().Attributes().PutStr("k8s.namespace.name", "ns-beta")
	r2.ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	var chunks []ptrace.Traces
	var keys [][]byte
	for key, data := range e.partitionData(td) {
		clone := ptrace.NewTraces()
		data.CopyTo(clone)
		chunks = append(chunks, clone)
		keys = append(keys, key)
	}

	require.Len(t, chunks, 2, "should yield one chunk per resource")
	require.Nil(t, keys[0], "key should be nil (no explicit partitioning)")
	require.Nil(t, keys[1], "key should be nil (no explicit partitioning)")

	require.Equal(t, 1, chunks[0].ResourceSpans().Len())
	require.Equal(t, 1, chunks[1].ResourceSpans().Len())

	v0, _ := chunks[0].ResourceSpans().At(0).Resource().Attributes().Get("k8s.namespace.name")
	v1, _ := chunks[1].ResourceSpans().At(0).Resource().Attributes().Get("k8s.namespace.name")
	require.Equal(t, "ns-alpha", v0.Str())
	require.Equal(t, "ns-beta", v1.Str())
}

func TestPartitionData_TopicFromAttributeSplitsProfiles(t *testing.T) {
	cfg := Config{TopicFromAttribute: "k8s.namespace.name"}
	e := &kafkaProfilesMessenger{config: cfg}

	pd := pprofile.NewProfiles()
	r1 := pd.ResourceProfiles().AppendEmpty()
	r1.Resource().Attributes().PutStr("k8s.namespace.name", "ns-alpha")
	r1.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	r2 := pd.ResourceProfiles().AppendEmpty()
	r2.Resource().Attributes().PutStr("k8s.namespace.name", "ns-beta")
	r2.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()

	var chunks []pprofile.Profiles
	var keys [][]byte
	for key, data := range e.partitionData(pd) {
		clone := pprofile.NewProfiles()
		data.CopyTo(clone)
		chunks = append(chunks, clone)
		keys = append(keys, key)
	}

	require.Len(t, chunks, 2, "should yield one chunk per resource")
	require.Nil(t, keys[0], "key should be nil (no explicit partitioning)")
	require.Nil(t, keys[1], "key should be nil (no explicit partitioning)")

	require.Equal(t, 1, chunks[0].ResourceProfiles().Len())
	require.Equal(t, 1, chunks[1].ResourceProfiles().Len())

	v0, _ := chunks[0].ResourceProfiles().At(0).Resource().Attributes().Get("k8s.namespace.name")
	v1, _ := chunks[1].ResourceProfiles().At(0).Resource().Attributes().Get("k8s.namespace.name")
	require.Equal(t, "ns-alpha", v0.Str())
	require.Equal(t, "ns-beta", v1.Str())
}

func TestPartitionData_NoSplitWithoutTopicFromAttribute(t *testing.T) {
	cfg := Config{} // no TopicFromAttribute, no partitioning
	e := &kafkaMetricsMessenger{config: cfg}

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty()
	md.ResourceMetrics().AppendEmpty()

	var count int
	for range e.partitionData(md) {
		count++
	}
	require.Equal(t, 1, count, "should yield entire batch as one chunk")
}

func TestMetricsPusher_topicFromAttribute_multiResource(t *testing.T) {
	// Two resources, each targeting a different topic via the same attribute key.
	topicAttr := "target.topic"
	topicA := "topic-alpha"
	topicB := "topic-beta"

	input := pmetric.NewMetrics()
	rm1 := input.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr(topicAttr, topicA)
	rm1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric.a")

	rm2 := input.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr(topicAttr, topicB)
	rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("metric.b")

	config := createDefaultConfig().(*Config)
	config.TopicFromAttribute = topicAttr
	// No PartitionMetricsByResourceAttributes -- this is the bug scenario.

	exp, fakeCluster := newKgoMockMetricsExporter(t, *config,
		componenttest.NewNopHost(), topicA, topicB)
	defer fakeCluster.Close()

	err := exp.exportData(t.Context(), input)
	require.NoError(t, err)

	// Fetch from topicA only -- it always has records regardless of fix state.
	// Fetching from topicB before the fix would hang (no records there).
	records := fetchKgoRecords(t, fakeCluster.ListenAddrs(), topicA, 1)
	require.Len(t, records, 1, "expected exactly 1 record on %s", topicA)

	// Deserialize the record and check how many resources it contains.
	md, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(records[0].Value)
	require.NoError(t, err)

	// Before fix: ResourceMetrics().Len() == 2 (both resources in one record).
	// After fix:  ResourceMetrics().Len() == 1 (only topicA's resource).
	require.Equal(t, 1, md.ResourceMetrics().Len(),
		"bug: record on %s contains %d resources (expected 1 after split)",
		topicA, md.ResourceMetrics().Len())

	// Verify the surviving resource has the correct attribute value.
	val, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get(topicAttr)
	require.True(t, ok, "resource must have attribute %q", topicAttr)
	require.Equal(t, topicA, val.Str(),
		"resource on %s must have attribute value %q", topicA, topicA)
}

func TestKafkaExporter_ComponentStatus(t *testing.T) {
	t.Run("when status is OK", func(t *testing.T) {
		statusChan := make(chan *componentstatus.Event, 3)
		reporter := &testStatusReporter{statusChan: statusChan}

		config := createDefaultConfig().(*Config)

		exp, fakeCluster := newKgoMockLogsExporter(t, *config, reporter, config.Logs.Topic)
		t.Cleanup(func() { fakeCluster.Close() })

		logs := testdata.GenerateLogs(1)
		require.NoError(t, exp.exportData(t.Context(), logs))

		select {
		case event := <-statusChan:
			assert.NoError(t, event.Err())
			assert.Equal(t, componentstatus.StatusOK, event.Status())
		default:
			require.Fail(t, "successful export should report StatusOK")
		}
	})

	t.Run("when status is RecoverablError", func(t *testing.T) {
		statusChan := make(chan *componentstatus.Event, 2)
		reporter := &testStatusReporter{statusChan: statusChan}

		config := createDefaultConfig().(*Config)

		exp, fakeCluster := newKgoMockLogsExporter(t, *config, reporter, config.Logs.Topic)
		fakeCluster.Close()

		logs := testdata.GenerateLogs(1)

		go func() {
			// exportData will block due to the cluster being unavailable.
			// It will unblock when the test completes.
			_ = exp.exportData(t.Context(), logs)
		}()

		select {
		case event := <-statusChan:
			assert.Error(t, event.Err())
			assert.Equal(t, componentstatus.StatusRecoverableError, event.Status())
		case <-time.After(2 * time.Minute):
			require.Fail(t, "export should report recoverable error")
		}
	})
}

type testStatusReporter struct {
	statusChan chan *componentstatus.Event
}

func (tsr *testStatusReporter) Report(event *componentstatus.Event) {
	tsr.statusChan <- event
}

func (*testStatusReporter) GetExtensions() map[component.ID]component.Component {
	return make(map[component.ID]component.Component)
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
		kgo.WithHooks(kafkaclient.NewStatusReporter(host)),
	}

	client, err := kafka.NewFranzSyncProducer(tb.Context(), host, kcfg,
		cfg.Producer, 1*time.Second, zap.NewNop(), kgoClientOpts...)
	require.NoError(tb, err, "failed to create kgo.Client with fake cluster addresses")

	messenger, err := exp.newMessenger(host) // messenger implements Marshaler[pmetric.Metrics]
	require.NoError(tb, err, "failed to create messenger for metrics")

	exp.messenger = messenger
	exp.producer = kafkaclient.NewFranzSyncProducer(client, cfg.IncludeMetadataKeys, cfg.RecordHeaders, cfg.Producer.MaxMessageBytes, nil)

	tb.Cleanup(func() { client.Close() })
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
