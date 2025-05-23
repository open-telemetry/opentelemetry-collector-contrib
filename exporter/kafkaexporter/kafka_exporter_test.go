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
	"go.opentelemetry.io/collector/consumer/consumererror"
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
	producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
		func(msg *sarama.ProducerMessage) error {
			if msg.Topic != "otlp_spans" {
				return fmt.Errorf(`expected topic "otlp_spans", got %q`, msg.Topic)
			}
			return nil
		},
	)

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

	expErr := errors.New("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	err := exp.exportData(context.Background(), testdata.GenerateTraces(2))
	assert.EqualError(t, err, expErr.Error())
}

func TestTracesPusher_conf_err(t *testing.T) {
	t.Run("should return permanent err on config error", func(t *testing.T) {
		expErr := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: expErr},
		}
		host := extensionsHost{
			component.MustNewID("trace_encoding"): ptraceMarshalerFuncExtension(func(ptrace.Traces) ([]byte, error) {
				return nil, prodErrs
			}),
		}
		config := createDefaultConfig().(*Config)
		config.Traces.Encoding = "trace_encoding"
		exp, _ := newMockTracesExporter(t, *config, host)

		err := exp.exportData(context.Background(), testdata.GenerateTraces(2))

		assert.True(t, consumererror.IsPermanent(err))
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
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
			func(msg *sarama.ProducerMessage) error {
				if msg.Key != nil {
					return errors.New("message key should be nil")
				}
				return nil
			},
		)

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
	producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
		func(msg *sarama.ProducerMessage) error {
			if msg.Topic != "otlp_metrics" {
				return fmt.Errorf(`expected topic "otlp_metrics", got %q`, msg.Topic)
			}
			return nil
		},
	)

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

	expErr := errors.New("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	err := exp.exportData(context.Background(), testdata.GenerateMetrics(2))
	assert.EqualError(t, err, expErr.Error())
}

func TestMetricsPusher_conf_err(t *testing.T) {
	t.Run("should return permanent err on config error", func(t *testing.T) {
		expErr := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: expErr},
		}
		host := extensionsHost{
			component.MustNewID("metric_encoding"): ptraceMarshalerFuncExtension(func(ptrace.Traces) ([]byte, error) {
				return nil, prodErrs
			}),
		}
		config := createDefaultConfig().(*Config)
		config.Traces.Encoding = "metric_encoding"
		exp, _ := newMockTracesExporter(t, *config, host)

		err := exp.exportData(context.Background(), testdata.GenerateTraces(2))

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
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
			func(msg *sarama.ProducerMessage) error {
				if msg.Key != nil {
					return errors.New("message key should be nil")
				}
				return nil
			},
		)

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
	producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
		func(msg *sarama.ProducerMessage) error {
			if msg.Topic != "otlp_logs" {
				return fmt.Errorf(`expected topic "otlp_logs", got %q`, msg.Topic)
			}
			return nil
		},
	)

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

	expErr := errors.New("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	err := exp.exportData(context.Background(), testdata.GenerateLogs(2))
	assert.EqualError(t, err, expErr.Error())
}

func TestLogsPusher_conf_err(t *testing.T) {
	t.Run("should return permanent err on config error", func(t *testing.T) {
		expErr := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: expErr},
		}
		host := extensionsHost{
			component.MustNewID("log_encoding"): ptraceMarshalerFuncExtension(func(ptrace.Traces) ([]byte, error) {
				return nil, prodErrs
			}),
		}
		config := createDefaultConfig().(*Config)
		config.Traces.Encoding = "log_encoding"
		exp, _ := newMockTracesExporter(t, *config, host)

		err := exp.exportData(context.Background(), testdata.GenerateTraces(2))

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
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(
			func(msg *sarama.ProducerMessage) error {
				if msg.Key != nil {
					return errors.New("message key should be nil")
				}
				return nil
			},
		)

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
			ctx:                topic.WithTopic(context.Background(), "context-topic"),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "resource-attr-val-1",
		},
		{
			name:               "Valid trace attribute, return topic name",
			topicFromAttribute: "resource-attr",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(context.Background(), "context-topic"),
			resource:           testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic:          "resource-attr-val-1",
		},
		{
			name:               "Valid log attribute, return topic name",
			topicFromAttribute: "resource-attr",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(context.Background(), "context-topic"),
			resource:           testdata.GenerateLogs(1).ResourceLogs(),
			wantTopic:          "resource-attr-val-1",
		},
		{
			name:               "Attribute not found",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                context.Background(),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "defaultTopic",
		},
		// Nonexistent attribute tests.
		{
			name:               "Valid metric context, return topic name",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(context.Background(), "context-topic"),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "context-topic",
		},
		{
			name:               "Valid trace context, return topic name",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(context.Background(), "context-topic"),
			resource:           testdata.GenerateTraces(1).ResourceSpans(),
			wantTopic:          "context-topic",
		},
		{
			name:               "Valid log context, return topic name",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                topic.WithTopic(context.Background(), "context-topic"),
			resource:           testdata.GenerateLogs(1).ResourceLogs(),
			wantTopic:          "context-topic",
		},
		// Generic known failure modes.
		{
			name:               "Attribute not found",
			topicFromAttribute: "nonexistent_attribute",
			signalCfg:          SignalConfig{Topic: "defaultTopic"},
			ctx:                context.Background(),
			resource:           testdata.GenerateMetrics(1).ResourceMetrics(),
			wantTopic:          "defaultTopic",
		},
		{
			name:      "TopicFromAttribute, return default topic",
			ctx:       context.Background(),
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
			ctx: client.NewContext(context.Background(),
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
			ctx: client.NewContext(context.Background(),
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
			ctx: client.NewContext(context.Background(),
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
			ctx: client.NewContext(context.Background(),
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
				topic = getTopic(tests[i].ctx, tests[i].signalCfg, tests[i].topicFromAttribute, r)
			case ptrace.ResourceSpansSlice:
				topic = getTopic(tests[i].ctx, tests[i].signalCfg, tests[i].topicFromAttribute, r)
			case plog.ResourceLogsSlice:
				topic = getTopic(tests[i].ctx, tests[i].signalCfg, tests[i].topicFromAttribute, r)
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

func TestWrapKafkaProducerError(t *testing.T) {
	t.Run("should return permanent error on configuration error", func(t *testing.T) {
		err := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.True(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return permanent error whne multiple configuration error", func(t *testing.T) {
		err := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
			&sarama.ProducerError{Err: err},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.True(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return not permanent error when at least one not configuration error", func(t *testing.T) {
		err := sarama.ConfigurationError("configuration error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
			&sarama.ProducerError{Err: errors.New("other producer error")},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.False(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return not permanent error on other producer error", func(t *testing.T) {
		err := errors.New("other producer error")
		prodErrs := sarama.ProducerErrors{
			&sarama.ProducerError{Err: err},
		}

		got := wrapKafkaProducerError(prodErrs)

		assert.False(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})

	t.Run("should return not permanent error when other error", func(t *testing.T) {
		err := errors.New("other error")

		got := wrapKafkaProducerError(err)

		assert.False(t, consumererror.IsPermanent(got))
		assert.Contains(t, got.Error(), err.Error())
	})
}
