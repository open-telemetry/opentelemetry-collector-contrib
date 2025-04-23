// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "cloud.google.com/go/pubsub/apiv1/pubsubpb"
	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter/internal/metadata"
)

const (
	defaultUUID      = "00000000-0000-0000-0000-000000000000"
	defaultProjectID = "my-project"
	defaultTopic     = "projects/my-project/topics/otlp"
)

func TestGetMessageAttributes(t *testing.T) {
	date := time.Date(2021, time.January, 1, 2, 3, 4, 5, time.UTC)

	t.Run("logs", func(t *testing.T) {
		exporter, _ := newTestExporter(t)

		gotAttributes, err := exporter.getMessageAttributes(otlpProtoLog, date)
		require.NoError(t, err)

		expectedAttributes := map[string]string{
			"ce-id":          "00000000-0000-0000-0000-000000000000",
			"ce-source":      "/opentelemetry/collector/googlecloudpubsub/latest",
			"ce-specversion": "1.0",
			"ce-time":        "2021-01-01T02:03:04.000000005Z",
			"ce-type":        "org.opentelemetry.otlp.logs.v1",
			"content-type":   "application/protobuf",
		}
		assert.Equal(t, expectedAttributes, gotAttributes)
	})

	t.Run("metrics", func(t *testing.T) {
		exporter, _ := newTestExporter(t)

		gotAttributes, err := exporter.getMessageAttributes(otlpProtoMetric, date)
		require.NoError(t, err)

		expectedAttributes := map[string]string{
			"ce-id":          "00000000-0000-0000-0000-000000000000",
			"ce-source":      "/opentelemetry/collector/googlecloudpubsub/latest",
			"ce-specversion": "1.0",
			"ce-time":        "2021-01-01T02:03:04.000000005Z",
			"ce-type":        "org.opentelemetry.otlp.metrics.v1",
			"content-type":   "application/protobuf",
		}
		assert.Equal(t, expectedAttributes, gotAttributes)
	})

	t.Run("traces", func(t *testing.T) {
		exporter, _ := newTestExporter(t)

		gotAttributes, err := exporter.getMessageAttributes(otlpProtoTrace, date)
		require.NoError(t, err)

		expectedAttributes := map[string]string{
			"ce-id":          "00000000-0000-0000-0000-000000000000",
			"ce-source":      "/opentelemetry/collector/googlecloudpubsub/latest",
			"ce-specversion": "1.0",
			"ce-time":        "2021-01-01T02:03:04.000000005Z",
			"ce-type":        "org.opentelemetry.otlp.traces.v1",
			"content-type":   "application/protobuf",
		}
		assert.Equal(t, expectedAttributes, gotAttributes)
	})

	t.Run("logs with compression", func(t *testing.T) {
		exporter, _ := newTestExporter(t, func(cfg *Config) {
			cfg.Compression = "gzip"
		})

		gotAttributes, err := exporter.getMessageAttributes(otlpProtoLog, date)
		require.NoError(t, err)

		expectedAttributes := map[string]string{
			"ce-id":            "00000000-0000-0000-0000-000000000000",
			"ce-source":        "/opentelemetry/collector/googlecloudpubsub/latest",
			"ce-specversion":   "1.0",
			"ce-time":          "2021-01-01T02:03:04.000000005Z",
			"ce-type":          "org.opentelemetry.otlp.logs.v1",
			"content-type":     "application/protobuf",
			"content-encoding": "gzip",
		}
		assert.Equal(t, expectedAttributes, gotAttributes)
	})
}

func TestExporterNoData(t *testing.T) {
	exporter, publisher := newTestExporter(t, func(config *Config) {
		config.Watermark.Behavior = "earliest"
	})

	ctx := context.Background()
	assert.NoError(t, exporter.consumeLogs(ctx, plog.NewLogs()))
	assert.NoError(t, exporter.consumeMetrics(ctx, pmetric.NewMetrics()))
	assert.NoError(t, exporter.consumeTraces(ctx, ptrace.NewTraces()))

	assert.Zero(t, publisher.requests)
}

func TestExporterClientError(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.ProjectID = defaultProjectID
	cfg.Topic = defaultTopic
	require.NoError(t, cfg.Validate())

	exporter := ensureExporter(exportertest.NewNopSettings(metadata.Type), cfg)
	exporter.makeClient = func(context.Context, *Config, string) (publisherClient, error) {
		return nil, errors.New("something went wrong")
	}

	require.Error(t, exporter.start(context.Background(), componenttest.NewNopHost()))
}

func TestExporterSimpleData(t *testing.T) {
	t.Run("logs", func(t *testing.T) {
		exporter, publisher := newTestExporter(t)

		logs := plog.NewLogs()
		logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message")

		require.NoError(t, exporter.consumeLogs(context.Background(), logs))
		require.Len(t, publisher.requests, 1)

		request := publisher.requests[0]
		assert.Equal(t, defaultTopic, request.Topic)
		assert.Len(t, request.Messages, 1)

		message := request.Messages[0]
		assert.NotEmpty(t, message.Data)
		assert.Subset(t, message.Attributes, map[string]string{
			"ce-type":      "org.opentelemetry.otlp.logs.v1",
			"content-type": "application/protobuf",
		})
	})

	t.Run("metrics", func(t *testing.T) {
		exporter, publisher := newTestExporter(t)

		metrics := pmetric.NewMetrics()
		metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("some.metric")
		metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)

		require.NoError(t, exporter.consumeMetrics(context.Background(), metrics))
		require.Len(t, publisher.requests, 1)

		request := publisher.requests[0]
		assert.Equal(t, defaultTopic, request.Topic)
		assert.Len(t, request.Messages, 1)

		message := request.Messages[0]
		assert.NotEmpty(t, message.Data)
		assert.Subset(t, message.Attributes, map[string]string{
			"ce-type":      "org.opentelemetry.otlp.metrics.v1",
			"content-type": "application/protobuf",
		})
	})

	t.Run("traces", func(t *testing.T) {
		exporter, publisher := newTestExporter(t)

		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("some span")

		require.NoError(t, exporter.consumeTraces(context.Background(), traces))
		require.Len(t, publisher.requests, 1)

		request := publisher.requests[0]
		assert.Equal(t, defaultTopic, request.Topic)
		assert.Len(t, request.Messages, 1)

		message := request.Messages[0]
		assert.NotEmpty(t, message.Data)
		assert.Subset(t, message.Attributes, map[string]string{
			"ce-type":      "org.opentelemetry.otlp.traces.v1",
			"content-type": "application/protobuf",
		})
	})
}

func TestExporterSimpleDataWithCompression(t *testing.T) {
	withCompression := func(config *Config) {
		config.Compression = "gzip"
	}

	t.Run("logs", func(t *testing.T) {
		exporter, publisher := newTestExporter(t, withCompression)

		logs := plog.NewLogs()
		logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message")

		require.NoError(t, exporter.consumeLogs(context.Background(), logs))
		require.Len(t, publisher.requests, 1)

		request := publisher.requests[0]
		assert.Equal(t, defaultTopic, request.Topic)
		assert.Len(t, request.Messages, 1)

		message := request.Messages[0]
		assert.NotEmpty(t, message.Data)
		assert.Subset(t, message.Attributes, map[string]string{
			"ce-id":            "00000000-0000-0000-0000-000000000000",
			"ce-source":        "/opentelemetry/collector/googlecloudpubsub/latest",
			"ce-specversion":   "1.0",
			"ce-type":          "org.opentelemetry.otlp.logs.v1",
			"content-type":     "application/protobuf",
			"content-encoding": "gzip",
		})
	})

	t.Run("metrics", func(t *testing.T) {
		exporter, publisher := newTestExporter(t, withCompression)

		metrics := pmetric.NewMetrics()
		metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		metric.SetName("some.metric")
		metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)

		require.NoError(t, exporter.consumeMetrics(context.Background(), metrics))
		require.Len(t, publisher.requests, 1)

		request := publisher.requests[0]
		assert.Equal(t, defaultTopic, request.Topic)
		assert.Len(t, request.Messages, 1)

		message := request.Messages[0]
		assert.NotEmpty(t, message.Data)
		assert.Subset(t, message.Attributes, map[string]string{
			"ce-type":          "org.opentelemetry.otlp.metrics.v1",
			"content-type":     "application/protobuf",
			"content-encoding": "gzip",
		})
	})

	t.Run("traces", func(t *testing.T) {
		exporter, publisher := newTestExporter(t, withCompression)

		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetName("some span")

		require.NoError(t, exporter.consumeTraces(context.Background(), traces))
		require.Len(t, publisher.requests, 1)

		request := publisher.requests[0]
		assert.Equal(t, defaultTopic, request.Topic)
		assert.Len(t, request.Messages, 1)

		message := request.Messages[0]
		assert.NotEmpty(t, message.Data)
		assert.Subset(t, message.Attributes, map[string]string{
			"ce-type":          "org.opentelemetry.otlp.traces.v1",
			"content-type":     "application/protobuf",
			"content-encoding": "gzip",
		})
	})
}

func TestExporterWithOrdering(t *testing.T) {
	const orderingKey = "ordering.key"
	withOrdering := func(cfg *Config) {
		cfg.Ordering.Enabled = true
		cfg.Ordering.FromResourceAttribute = orderingKey
		cfg.Ordering.RemoveResourceAttribute = true
	}

	ctx := context.Background()

	t.Run("logs", func(t *testing.T) {
		exporter, publisher := newTestExporter(t, withOrdering)

		logs := plog.NewLogs()
		{
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message without ordering key 1")
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message without ordering key 2")
		}
		{
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			resourceLogs.Resource().Attributes().PutStr(orderingKey, "")
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message without ordering key 1")
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message without ordering key 2")
		}
		{
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			resourceLogs.Resource().Attributes().PutStr(orderingKey, "value 1")
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message 1 with ordering key 1")
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message 2 with ordering key 1")
		}
		{
			resourceLogs := logs.ResourceLogs().AppendEmpty()
			resourceLogs.Resource().Attributes().PutStr(orderingKey, "value 2")
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message 1 with ordering key 2")
			resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("some log message 2 with ordering key 2")
		}

		require.NoError(t, exporter.consumeLogs(ctx, logs))
		require.Len(t, publisher.requests, 3, "one publish call per ordering key should've been made")

		var orderingKeyValues []string
		for _, request := range publisher.requests {
			assert.Equal(t, defaultTopic, request.Topic)
			assert.Len(t, request.Messages, 1)

			for _, msg := range request.Messages {
				orderingKeyValues = append(orderingKeyValues, msg.OrderingKey)

				assert.NotEmpty(t, msg.Data)
				assert.NotEmpty(t, msg.Attributes)
			}
		}
		assert.ElementsMatch(t, orderingKeyValues, []string{"", "value 1", "value 2"})
	})

	t.Run("metrics", func(t *testing.T) {
		exporter, publisher := newTestExporter(t, withOrdering)

		metrics := pmetric.NewMetrics()
		{
			resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
			metric := resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName("some.metric")
			metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)
			metric.Gauge().DataPoints().AppendEmpty().SetIntValue(24)
		}
		{
			resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
			resourceMetrics.Resource().Attributes().PutStr(orderingKey, "")
			metric := resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName("some.metric")
			metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)
			metric.Gauge().DataPoints().AppendEmpty().SetIntValue(24)
		}
		{
			resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
			resourceMetrics.Resource().Attributes().PutStr(orderingKey, "value 1")
			metric := resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName("some.metric")
			metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)
			metric.Gauge().DataPoints().AppendEmpty().SetIntValue(24)
		}
		{
			resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
			resourceMetrics.Resource().Attributes().PutStr(orderingKey, "value 2")
			metric := resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			metric.SetName("some.metric")
			metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(42)
			metric.Gauge().DataPoints().AppendEmpty().SetIntValue(24)
		}

		require.NoError(t, exporter.consumeMetrics(ctx, metrics))
		require.Len(t, publisher.requests, 3, "one publish call per ordering key should've been made")

		var orderingKeyValues []string
		for _, request := range publisher.requests {
			assert.Equal(t, defaultTopic, request.Topic)
			assert.Len(t, request.Messages, 1)

			for _, msg := range request.Messages {
				orderingKeyValues = append(orderingKeyValues, msg.OrderingKey)

				assert.NotEmpty(t, msg.Data)
				assert.NotEmpty(t, msg.Attributes)
			}
		}
		assert.ElementsMatch(t, orderingKeyValues, []string{"", "value 1", "value 2"})
	})

	t.Run("traces", func(t *testing.T) {
		exporter, publisher := newTestExporter(t, withOrdering)

		traces := ptrace.NewTraces()
		{
			resourceSpans := traces.ResourceSpans().AppendEmpty()
			span := resourceSpans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetName("some span 1")
		}
		{
			resourceSpans := traces.ResourceSpans().AppendEmpty()
			resourceSpans.Resource().Attributes().PutStr(orderingKey, "")
			span := resourceSpans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetName("some span 1")
		}
		{
			resourceSpans := traces.ResourceSpans().AppendEmpty()
			resourceSpans.Resource().Attributes().PutStr(orderingKey, "value 1")
			span := resourceSpans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetName("some span 2")
		}
		{
			resourceSpans := traces.ResourceSpans().AppendEmpty()
			resourceSpans.Resource().Attributes().PutStr(orderingKey, "value 2")
			span := resourceSpans.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetName("some span 3")
		}

		require.NoError(t, exporter.consumeTraces(ctx, traces))
		require.Len(t, publisher.requests, 3, "one publish call per ordering key should've been made")

		var orderingKeyValues []string
		for _, request := range publisher.requests {
			assert.Equal(t, defaultTopic, request.Topic)
			assert.Len(t, request.Messages, 1)

			for _, msg := range request.Messages {
				orderingKeyValues = append(orderingKeyValues, msg.OrderingKey)

				assert.NotEmpty(t, msg.Data)
				assert.NotEmpty(t, msg.Attributes)
			}
		}
		assert.ElementsMatch(t, orderingKeyValues, []string{"", "value 1", "value 2"})
	})
}

// Helpers

func newTestExporter(t *testing.T, options ...func(*Config)) (*pubsubExporter, *mockPublisher) {
	t.Helper()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.ProjectID = defaultProjectID
	cfg.Topic = defaultTopic
	for _, option := range options {
		option(cfg)
	}
	require.NoError(t, cfg.Validate())

	exporter := ensureExporter(exportertest.NewNopSettings(metadata.Type), cfg)
	publisher := &mockPublisher{}
	exporter.makeClient = func(context.Context, *Config, string) (publisherClient, error) {
		return publisher, nil
	}
	exporter.makeUUID = func() (uuid.UUID, error) {
		return uuid.Parse(defaultUUID)
	}

	require.NoError(t, exporter.start(context.Background(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, exporter.shutdown(context.Background())) })

	return exporter, publisher
}

type mockPublisher struct {
	requests []*pb.PublishRequest
}

func (m *mockPublisher) Publish(_ context.Context, request *pb.PublishRequest, _ ...gax.CallOption) (*pb.PublishResponse, error) {
	m.requests = append(m.requests, request)
	return &pb.PublishResponse{}, nil
}

func (m *mockPublisher) Close() error {
	return nil
}
