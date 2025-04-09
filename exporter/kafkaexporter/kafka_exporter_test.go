// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestTracesPusher(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())
	producer.ExpectSendMessageAndSucceed()

	err := exp.exportData(context.Background(), testdata.GenerateTraces(2))
	require.NoError(t, err)
}

func TestTracesPusher_attr(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.TopicFromAttribute = "kafka_topic"
	exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())
	producer.ExpectSendMessageAndSucceed()

	err := exp.exportData(context.Background(), testdata.GenerateTraces(2))
	require.NoError(t, err)
}

func TestTracesPusher_ctx(t *testing.T) {
	t.Run("WithTopic", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageAndSucceed()

		err := exp.exportData(topic.WithTopic(context.Background(), "my_topic"), testdata.GenerateTraces(2))
		require.NoError(t, err)
	})
	t.Run("WithMetadata", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.IncludeMetadataKeys = []string{"x-tenant-id", "x-request-ids"}
		exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
			assert.Equal(t, []sarama.RecordHeader{
				{Key: []byte("x-tenant-id"), Value: []byte("my_tenant_id")},
				{Key: []byte("x-request-ids"), Value: []byte("987654321")},
				{Key: []byte("x-request-ids"), Value: []byte("0187262")},
			}, pm.Headers)
			return nil
		})
		t.Cleanup(func() {
			require.NoError(t, exp.Close(context.Background()))
		})
		ctx := client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":    {"my_tenant_id"},
				"x-request-ids":  {"987654321", "0187262"},
				"discarded-meta": {"my-meta"}, // This will be ignored.
			}),
		})
		err := exp.exportData(ctx, testdata.GenerateTraces(10))
		require.NoError(t, err)
	})
	t.Run("WithMetadataDisabled", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
			assert.Nil(t, pm.Headers)
			return nil
		})
		t.Cleanup(func() {
			require.NoError(t, exp.Close(context.Background()))
		})
		ctx := client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":    {"my_tenant_id"},
				"x-request-ids":  {"123456789", "0187262"},
				"discarded-meta": {"my-meta"},
			}),
		})
		err := exp.exportData(ctx, testdata.GenerateTraces(5))
		require.NoError(t, err)
	})
}

func TestTracesPusher_err(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())

	expErr := fmt.Errorf("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	err := exp.exportData(context.Background(), testdata.GenerateTraces(2))
	assert.EqualError(t, err, expErr.Error())
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
	exp, _ := newMockTracesExporter(t, *config, host)

	err := exp.exportData(context.Background(), testdata.GenerateTraces(2))
	assert.ErrorContains(t, err, marshalErr.Error())
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
		exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageAndSucceed()

		err := exp.exportData(context.Background(), input)
		require.NoError(t, err)
	})
	t.Run("jaeger_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.Traces.Encoding = "jaeger_json"
		exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())

		// Jaeger encodings produce one message per span,
		// and each one will have the trace ID as the key.
		var keys [][]byte
		for i := 0; i < 4; i++ {
			producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
				func(msg *sarama.ProducerMessage) error {
					key, err := msg.Key.Encode()
					require.NoError(t, err)
					keys = append(keys, key)
					return nil
				},
			)
		}

		err := exp.exportData(context.Background(), input)
		require.NoError(t, err)
		require.Len(t, keys, 4)
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
		exp, producer := newMockTracesExporter(t, *config, componenttest.NewNopHost())

		// We should get one message per ResourceSpans,
		// even if they have the same service name.
		var keys [][]byte
		var traces []ptrace.Traces
		for i := 0; i < 2; i++ {
			producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
				func(msg *sarama.ProducerMessage) error {
					value, err := msg.Value.Encode()
					require.NoError(t, err)

					output, err := (&ptrace.ProtoUnmarshaler{}).UnmarshalTraces(value)
					require.NoError(t, err)
					traces = append(traces, output)

					key, err := msg.Key.Encode()
					require.NoError(t, err)
					keys = append(keys, key)
					return nil
				},
			)
		}

		err := exp.exportData(context.Background(), input)
		require.NoError(t, err)

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

func TestMetricsDataPusher(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())
	producer.ExpectSendMessageAndSucceed()

	err := exp.exportData(context.Background(), testdata.GenerateMetrics(2))
	require.NoError(t, err)
}

func TestMetricsDataPusher_attr(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.TopicFromAttribute = "kafka_topic"
	exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())
	producer.ExpectSendMessageAndSucceed()

	err := exp.exportData(context.Background(), testdata.GenerateMetrics(2))
	require.NoError(t, err)
}

func TestMetricsDataPusher_ctx(t *testing.T) {
	t.Run("WithTopic", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageAndSucceed()

		err := exp.exportData(topic.WithTopic(context.Background(), "my_topic"), testdata.GenerateMetrics(2))
		require.NoError(t, err)
	})
	t.Run("WithMetadata", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.IncludeMetadataKeys = []string{"x-tenant-id", "x-request-ids"}
		exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
			assert.Equal(t, []sarama.RecordHeader{
				{Key: []byte("x-tenant-id"), Value: []byte("my_tenant_id")},
				{Key: []byte("x-request-ids"), Value: []byte("123456789")},
				{Key: []byte("x-request-ids"), Value: []byte("123141")},
			}, pm.Headers)
			return nil
		})
		t.Cleanup(func() {
			require.NoError(t, exp.Close(context.Background()))
		})
		ctx := client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":    {"my_tenant_id"},
				"x-request-ids":  {"123456789", "123141"},
				"discarded-meta": {"my-meta"}, // This will be ignored.
			}),
		})
		err := exp.exportData(ctx, testdata.GenerateMetrics(10))
		require.NoError(t, err)
	})
	t.Run("WithMetadataDisabled", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
			assert.Nil(t, pm.Headers)
			return nil
		})
		t.Cleanup(func() {
			require.NoError(t, exp.Close(context.Background()))
		})
		ctx := client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":    {"my_tenant_id"},
				"x-request-ids":  {"123456789", "123141"},
				"discarded-meta": {"my-meta"},
			}),
		})
		err := exp.exportData(ctx, testdata.GenerateMetrics(5))
		require.NoError(t, err)
	})
}

func TestMetricsPusher_err(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())

	expErr := fmt.Errorf("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	err := exp.exportData(context.Background(), testdata.GenerateMetrics(2))
	assert.EqualError(t, err, expErr.Error())
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
	exp, _ := newMockMetricsExporter(t, *config, host)

	err := exp.exportData(context.Background(), testdata.GenerateMetrics(2))
	assert.ErrorContains(t, err, marshalErr.Error())
}

func TestMetricsPusher_partitioning(t *testing.T) {
	input := pmetric.NewMetrics()
	for _, serviceName := range []string{"service1", "service1", "service2"} {
		resourceMetrics := testdata.GenerateMetrics(1).ResourceMetrics().At(0)
		resourceMetrics.Resource().Attributes().PutStr("service.name", serviceName)
		resourceMetrics.CopyTo(input.ResourceMetrics().AppendEmpty())
	}

	t.Run("default_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageAndSucceed()

		err := exp.exportData(context.Background(), input)
		require.NoError(t, err)
	})
	t.Run("resource_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.PartitionMetricsByResourceAttributes = true
		exp, producer := newMockMetricsExporter(t, *config, componenttest.NewNopHost())

		// We should get one message per ResourceMetrics,
		// even if they have the same service name.
		var keys [][]byte
		for i := 0; i < 3; i++ {
			producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
				func(msg *sarama.ProducerMessage) error {
					value, err := msg.Value.Encode()
					require.NoError(t, err)

					output, err := (&pmetric.ProtoUnmarshaler{}).UnmarshalMetrics(value)
					require.NoError(t, err)

					require.Equal(t, 1, output.ResourceMetrics().Len())
					assert.NoError(t, pmetrictest.CompareResourceMetrics(
						input.ResourceMetrics().At(i),
						output.ResourceMetrics().At(0),
					))

					key, err := msg.Key.Encode()
					require.NoError(t, err)
					keys = append(keys, key)
					return nil
				},
			)
		}

		err := exp.exportData(context.Background(), input)
		require.NoError(t, err)

		require.Len(t, keys, 3)
		assert.NotEmpty(t, keys[0])
		assert.Equal(t, keys[0], keys[1])
		assert.NotEqual(t, keys[0], keys[2])
	})
}

func TestLogsDataPusher(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())
	producer.ExpectSendMessageAndSucceed()

	err := exp.exportData(context.Background(), testdata.GenerateLogs(2))
	require.NoError(t, err)
}

func TestLogsDataPusher_attr(t *testing.T) {
	config := createDefaultConfig().(*Config)
	config.TopicFromAttribute = "kafka_topic"
	exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())
	producer.ExpectSendMessageAndSucceed()

	err := exp.exportData(context.Background(), testdata.GenerateLogs(2))
	require.NoError(t, err)
}

func TestLogsDataPusher_ctx(t *testing.T) {
	t.Run("WithTopic", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageAndSucceed()

		err := exp.exportData(topic.WithTopic(context.Background(), "my_topic"), testdata.GenerateLogs(2))
		require.NoError(t, err)
	})
	t.Run("WithMetadata", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.IncludeMetadataKeys = []string{"x-tenant-id", "x-request-ids"}
		exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
			assert.Equal(t, []sarama.RecordHeader{
				{Key: []byte("x-tenant-id"), Value: []byte("my_tenant_id")},
				{Key: []byte("x-request-ids"), Value: []byte("123456789")},
				{Key: []byte("x-request-ids"), Value: []byte("123141")},
			}, pm.Headers)
			return nil
		})
		t.Cleanup(func() {
			require.NoError(t, exp.Close(context.Background()))
		})
		ctx := client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":    {"my_tenant_id"},
				"x-request-ids":  {"123456789", "123141"},
				"discarded-meta": {"my-meta"}, // This will be ignored.
			}),
		})
		err := exp.exportData(ctx, testdata.GenerateLogs(10))
		require.NoError(t, err)
	})
	t.Run("WithMetadataDisabled", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
			assert.Nil(t, pm.Headers)
			return nil
		})
		t.Cleanup(func() {
			require.NoError(t, exp.Close(context.Background()))
		})
		ctx := client.NewContext(context.Background(), client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				"x-tenant-id":    {"my_tenant_id"},
				"x-request-ids":  {"123456789", "123141"},
				"discarded-meta": {"my-meta"},
			}),
		})
		err := exp.exportData(ctx, testdata.GenerateLogs(5))
		require.NoError(t, err)
	})
}

func TestLogsPusher_err(t *testing.T) {
	config := createDefaultConfig().(*Config)
	exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())

	expErr := fmt.Errorf("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	err := exp.exportData(context.Background(), testdata.GenerateLogs(2))
	assert.EqualError(t, err, expErr.Error())
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
	exp, _ := newMockLogsExporter(t, *config, host)

	err := exp.exportData(context.Background(), testdata.GenerateLogs(2))
	assert.ErrorContains(t, err, marshalErr.Error())
}

func TestLogsPusher_partitioning(t *testing.T) {
	input := plog.NewLogs()
	for _, serviceName := range []string{"service1", "service1", "service2"} {
		resourceLogs := testdata.GenerateLogs(1).ResourceLogs().At(0)
		resourceLogs.Resource().Attributes().PutStr("service.name", serviceName)
		resourceLogs.CopyTo(input.ResourceLogs().AppendEmpty())
	}

	t.Run("default_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())
		producer.ExpectSendMessageAndSucceed()

		err := exp.exportData(context.Background(), input)
		require.NoError(t, err)
	})
	t.Run("resource_partitioning", func(t *testing.T) {
		config := createDefaultConfig().(*Config)
		config.PartitionLogsByResourceAttributes = true
		exp, producer := newMockLogsExporter(t, *config, componenttest.NewNopHost())

		// We should get one message per ResourceLogs,
		// even if they have the same service name.
		var keys [][]byte
		for i := 0; i < 3; i++ {
			producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
				func(msg *sarama.ProducerMessage) error {
					value, err := msg.Value.Encode()
					require.NoError(t, err)

					output, err := (&plog.ProtoUnmarshaler{}).UnmarshalLogs(value)
					require.NoError(t, err)

					require.Equal(t, 1, output.ResourceLogs().Len())
					assert.NoError(t, plogtest.CompareResourceLogs(
						input.ResourceLogs().At(i),
						output.ResourceLogs().At(0),
					))

					key, err := msg.Key.Encode()
					require.NoError(t, err)
					keys = append(keys, key)
					return nil
				},
			)
		}

		err := exp.exportData(context.Background(), input)
		require.NoError(t, err)

		require.Len(t, keys, 3)
		assert.NotEmpty(t, keys[0])
		assert.Equal(t, keys[0], keys[1])
		assert.NotEqual(t, keys[0], keys[2])
	})
}

func Test_GetTopic(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		ctx       context.Context
		resource  any
		wantTopic string
	}{
		{
			name: "Valid metric attribute, return topic name",
			cfg: Config{
				TopicFromAttribute: "resource-attr",
				Topic:              "defaultTopic",
			},
			ctx:       topic.WithTopic(context.Background(), "context-topic"),
			resource:  testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic: "resource-attr-val-1",
		},
		{
			name: "Valid trace attribute, return topic name",
			cfg: Config{
				TopicFromAttribute: "resource-attr",
				Topic:              "defaultTopic",
			},
			ctx:       topic.WithTopic(context.Background(), "context-topic"),
			resource:  testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic: "resource-attr-val-1",
		},
		{
			name: "Valid log attribute, return topic name",
			cfg: Config{
				TopicFromAttribute: "resource-attr",
				Topic:              "defaultTopic",
			},
			ctx:       topic.WithTopic(context.Background(), "context-topic"),
			resource:  testdata.GenerateLogs(1).ResourceLogs(),
			wantTopic: "resource-attr-val-1",
		},
		{
			name: "Attribute not found",
			cfg: Config{
				TopicFromAttribute: "nonexistent_attribute",
				Topic:              "defaultTopic",
			},
			ctx:       context.Background(),
			resource:  testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic: "defaultTopic",
		},

		{
			name: "Valid metric context, return topic name",
			cfg: Config{
				TopicFromAttribute: "nonexistent_attribute",
				Topic:              "defaultTopic",
			},
			ctx:       topic.WithTopic(context.Background(), "context-topic"),
			resource:  testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic: "context-topic",
		},
		{
			name: "Valid trace context, return topic name",
			cfg: Config{
				TopicFromAttribute: "nonexistent_attribute",
				Topic:              "defaultTopic",
			},
			ctx:       topic.WithTopic(context.Background(), "context-topic"),
			resource:  testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic: "context-topic",
		},
		{
			name: "Valid log context, return topic name",
			cfg: Config{
				TopicFromAttribute: "nonexistent_attribute",
				Topic:              "defaultTopic",
			},
			ctx:       topic.WithTopic(context.Background(), "context-topic"),
			resource:  testdata.GenerateLogs(1).ResourceLogs(),
			wantTopic: "context-topic",
		},

		{
			name: "Attribute not found",
			cfg: Config{
				TopicFromAttribute: "nonexistent_attribute",
				Topic:              "defaultTopic",
			},
			ctx:       context.Background(),
			resource:  testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic: "defaultTopic",
		},
		{
			name: "TopicFromAttribute, return default topic",
			cfg: Config{
				Topic: "defaultTopic",
			},
			ctx:       context.Background(),
			resource:  testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic: "defaultTopic",
		},
	}

	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			topic := ""
			switch r := tests[i].resource.(type) {
			case pmetric.ResourceMetricsSlice:
				topic = getTopic(tests[i].ctx, &tests[i].cfg, r)
			case ptrace.ResourceSpansSlice:
				topic = getTopic(tests[i].ctx, &tests[i].cfg, r)
			case plog.ResourceLogsSlice:
				topic = getTopic(tests[i].ctx, &tests[i].cfg, r)
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

func (f ptraceMarshalerFuncExtension) Start(context.Context, component.Host) error {
	return nil
}

func (f ptraceMarshalerFuncExtension) Shutdown(context.Context) error {
	return nil
}

type pmetricMarshalerFuncExtension func(pmetric.Metrics) ([]byte, error)

func (f pmetricMarshalerFuncExtension) MarshalMetrics(td pmetric.Metrics) ([]byte, error) {
	return f(td)
}

func (f pmetricMarshalerFuncExtension) Start(context.Context, component.Host) error {
	return nil
}

func (f pmetricMarshalerFuncExtension) Shutdown(context.Context) error {
	return nil
}

type plogMarshalerFuncExtension func(plog.Logs) ([]byte, error)

func (f plogMarshalerFuncExtension) MarshalLogs(td plog.Logs) ([]byte, error) {
	return f(td)
}

func (f plogMarshalerFuncExtension) Start(context.Context, component.Host) error {
	return nil
}

func (f plogMarshalerFuncExtension) Shutdown(context.Context) error {
	return nil
}

func newMockTracesExporter(t *testing.T, cfg Config, host component.Host) (*kafkaExporter[ptrace.Traces], *mocks.SyncProducer) {
	exp := newTracesExporter(cfg, exportertest.NewNopSettings(metadata.Type))

	// Fake starting the exporter.
	messager, err := exp.newMessager(host)
	require.NoError(t, err)
	exp.messager = messager

	// Create a mock producer.
	producer := mocks.NewSyncProducer(t, sarama.NewConfig())
	exp.producer = producer

	t.Cleanup(func() {
		assert.NoError(t, exp.Close(context.Background()))
	})
	return exp, producer
}

func newMockMetricsExporter(t *testing.T, cfg Config, host component.Host) (*kafkaExporter[pmetric.Metrics], *mocks.SyncProducer) {
	exp := newMetricsExporter(cfg, exportertest.NewNopSettings(metadata.Type))

	// Fake starting the exporter.
	messager, err := exp.newMessager(host)
	require.NoError(t, err)
	exp.messager = messager

	// Create a mock producer.
	producer := mocks.NewSyncProducer(t, sarama.NewConfig())
	exp.producer = producer

	t.Cleanup(func() {
		assert.NoError(t, exp.Close(context.Background()))
	})
	return exp, producer
}

func newMockLogsExporter(t *testing.T, cfg Config, host component.Host) (*kafkaExporter[plog.Logs], *mocks.SyncProducer) {
	exp := newLogsExporter(cfg, exportertest.NewNopSettings(metadata.Type))

	// Fake starting the exporter.
	messager, err := exp.newMessager(host)
	require.NoError(t, err)
	exp.messager = messager

	// Create a mock producer.
	producer := mocks.NewSyncProducer(t, sarama.NewConfig())
	exp.producer = producer

	t.Cleanup(func() {
		assert.NoError(t, exp.Close(context.Background()))
	})
	return exp, producer
}
