// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"fmt"
	"testing"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic"
)

func TestNewExporter_err_version(t *testing.T) {
	c := Config{ProtocolVersion: "0.0.0", Encoding: defaultEncoding}
	texp := newTracesExporter(c, exportertest.NewNopSettings(metadata.Type))
	err := texp.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewExporter_err_encoding(t *testing.T) {
	c := Config{Encoding: "foo"}
	texp := newTracesExporter(c, exportertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, texp)
	err := texp.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewMetricsExporter_err_version(t *testing.T) {
	c := Config{ProtocolVersion: "0.0.0", Encoding: defaultEncoding}
	mexp := newMetricsExporter(c, exportertest.NewNopSettings(metadata.Type))
	err := mexp.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewMetricsExporter_err_encoding(t *testing.T) {
	c := Config{Encoding: "bar"}
	mexp := newMetricsExporter(c, exportertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, mexp)
	err := mexp.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewMetricsExporter_err_traces_encoding(t *testing.T) {
	c := Config{Encoding: "jaeger_proto"}
	mexp := newMetricsExporter(c, exportertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, mexp)
	err := mexp.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestMetricsExporter_encoding_extension(t *testing.T) {
	c := Config{
		Encoding: "metrics_encoding",
	}
	texp := newMetricsExporter(c, exportertest.NewNopSettings(metadata.Type))
	require.NotNil(t, texp)
	err := texp.start(context.Background(), &testComponentHost{})
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), errUnrecognizedEncoding.Error())
}

func TestNewLogsExporter_err_version(t *testing.T) {
	c := Config{ProtocolVersion: "0.0.0", Encoding: defaultEncoding}
	lexp := newLogsExporter(c, exportertest.NewNopSettings(metadata.Type))
	require.NotNil(t, lexp)
	err := lexp.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
}

func TestNewLogsExporter_err_encoding(t *testing.T) {
	c := Config{Encoding: "bar"}
	lexp := newLogsExporter(c, exportertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, lexp)
	err := lexp.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestNewLogsExporter_err_traces_encoding(t *testing.T) {
	c := Config{Encoding: "jaeger_proto"}
	lexp := newLogsExporter(c, exportertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, lexp)
	err := lexp.start(context.Background(), componenttest.NewNopHost())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
}

func TestLogsExporter_encoding_extension(t *testing.T) {
	c := Config{
		Encoding: "logs_encoding",
	}
	texp := newLogsExporter(c, exportertest.NewNopSettings(metadata.Type))
	require.NotNil(t, texp)
	err := texp.start(context.Background(), &testComponentHost{})
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), errUnrecognizedEncoding.Error())
}

func TestNewExporter_err_compression(t *testing.T) {
	c := Config{
		Encoding: defaultEncoding,
		Producer: Producer{
			Compression: "idk",
		},
	}
	texp := newTracesExporter(c, exportertest.NewNopSettings(metadata.Type))
	require.NotNil(t, texp)
	err := texp.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.ErrorContains(t, err, "producer.compression should be one of 'none', 'gzip', 'snappy', 'lz4', or 'zstd'. configured value idk")
}

func TestTracesExporter_encoding_extension(t *testing.T) {
	c := Config{
		Encoding: "traces_encoding",
	}
	texp := newTracesExporter(c, exportertest.NewNopSettings(metadata.Type))
	require.NotNil(t, texp)
	err := texp.start(context.Background(), &testComponentHost{})
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), errUnrecognizedEncoding.Error())
}

func TestTracesPusher(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaTracesProducer{
		producer:  producer,
		marshaler: newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.tracesPusher(context.Background(), testdata.GenerateTraces(2))
	require.NoError(t, err)
}

func TestTracesPusher_attr(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaTracesProducer{
		cfg: Config{
			TopicFromAttribute: "kafka_topic",
		},
		producer:  producer,
		marshaler: newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.tracesPusher(context.Background(), testdata.GenerateTraces(2))
	require.NoError(t, err)
}

func TestTracesPusher_ctx(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaTracesProducer{
		producer:  producer,
		marshaler: newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.tracesPusher(topic.WithTopic(context.Background(), "my_topic"), testdata.GenerateTraces(2))
	require.NoError(t, err)
}

func TestTracesPusher_err(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	expErr := fmt.Errorf("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	p := kafkaTracesProducer{
		producer:  producer,
		marshaler: newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultEncoding, false),
		logger:    zap.NewNop(),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	td := testdata.GenerateTraces(2)
	err := p.tracesPusher(context.Background(), td)
	assert.EqualError(t, err, expErr.Error())
}

func TestTracesPusher_marshal_error(t *testing.T) {
	expErr := fmt.Errorf("failed to marshal")
	p := kafkaTracesProducer{
		marshaler: &tracesErrorMarshaler{err: expErr},
		logger:    zap.NewNop(),
	}
	td := testdata.GenerateTraces(2)
	err := p.tracesPusher(context.Background(), td)
	assert.ErrorContains(t, err, expErr.Error())
}

func TestMetricsDataPusher(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaMetricsProducer{
		producer:  producer,
		marshaler: newPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.metricsDataPusher(context.Background(), testdata.GenerateMetrics(2))
	require.NoError(t, err)
}

func TestMetricsDataPusher_attr(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaMetricsProducer{
		cfg: Config{
			TopicFromAttribute: "kafka_topic",
		},
		producer:  producer,
		marshaler: newPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.metricsDataPusher(context.Background(), testdata.GenerateMetrics(2))
	require.NoError(t, err)
}

func TestMetricsDataPusher_ctx(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaMetricsProducer{
		producer:  producer,
		marshaler: newPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.metricsDataPusher(topic.WithTopic(context.Background(), "my_topic"), testdata.GenerateMetrics(2))
	require.NoError(t, err)
}

func TestMetricsDataPusher_err(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	expErr := fmt.Errorf("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	p := kafkaMetricsProducer{
		producer:  producer,
		marshaler: newPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}, defaultEncoding, false),
		logger:    zap.NewNop(),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	md := testdata.GenerateMetrics(2)
	err := p.metricsDataPusher(context.Background(), md)
	assert.EqualError(t, err, expErr.Error())
}

func TestMetricsDataPusher_marshal_error(t *testing.T) {
	expErr := fmt.Errorf("failed to marshal")
	p := kafkaMetricsProducer{
		marshaler: &metricsErrorMarshaler{err: expErr},
		logger:    zap.NewNop(),
	}
	md := testdata.GenerateMetrics(2)
	err := p.metricsDataPusher(context.Background(), md)
	assert.ErrorContains(t, err, expErr.Error())
}

func TestLogsDataPusher(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaLogsProducer{
		producer:  producer,
		marshaler: newPdataLogsMarshaler(&plog.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.logsDataPusher(context.Background(), testdata.GenerateLogs(1))
	require.NoError(t, err)
}

func TestLogsDataPusher_attr(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaLogsProducer{
		cfg: Config{
			TopicFromAttribute: "kafka_topic",
		},
		producer:  producer,
		marshaler: newPdataLogsMarshaler(&plog.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.logsDataPusher(context.Background(), testdata.GenerateLogs(1))
	require.NoError(t, err)
}

func TestLogsDataPusher_ctx(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	producer.ExpectSendMessageAndSucceed()

	p := kafkaLogsProducer{
		producer:  producer,
		marshaler: newPdataLogsMarshaler(&plog.ProtoMarshaler{}, defaultEncoding, false),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	err := p.logsDataPusher(topic.WithTopic(context.Background(), "my_topic"), testdata.GenerateLogs(1))
	require.NoError(t, err)
}

func TestLogsDataPusher_err(t *testing.T) {
	c := sarama.NewConfig()
	producer := mocks.NewSyncProducer(t, c)
	expErr := fmt.Errorf("failed to send")
	producer.ExpectSendMessageAndFail(expErr)

	p := kafkaLogsProducer{
		producer:  producer,
		marshaler: newPdataLogsMarshaler(&plog.ProtoMarshaler{}, defaultEncoding, false),
		logger:    zap.NewNop(),
	}
	t.Cleanup(func() {
		require.NoError(t, p.Close(context.Background()))
	})
	ld := testdata.GenerateLogs(1)
	err := p.logsDataPusher(context.Background(), ld)
	assert.EqualError(t, err, expErr.Error())
}

func TestLogsDataPusher_marshal_error(t *testing.T) {
	expErr := fmt.Errorf("failed to marshal")
	p := kafkaLogsProducer{
		marshaler: &logsErrorMarshaler{err: expErr},
		logger:    zap.NewNop(),
	}
	ld := testdata.GenerateLogs(1)
	err := p.logsDataPusher(context.Background(), ld)
	assert.ErrorContains(t, err, expErr.Error())
}

type tracesErrorMarshaler struct {
	err error
}

type metricsErrorMarshaler struct {
	err error
}

type logsErrorMarshaler struct {
	err error
}

func (e metricsErrorMarshaler) Marshal(_ pmetric.Metrics, _ string) ([]*sarama.ProducerMessage, error) {
	return nil, e.err
}

func (e metricsErrorMarshaler) Encoding() string {
	panic("implement me")
}

var _ TracesMarshaler = (*tracesErrorMarshaler)(nil)

func (e tracesErrorMarshaler) Marshal(_ ptrace.Traces, _ string) ([]*sarama.ProducerMessage, error) {
	return nil, e.err
}

func (e tracesErrorMarshaler) Encoding() string {
	panic("implement me")
}

func (e logsErrorMarshaler) Marshal(_ plog.Logs, _ string) ([]*sarama.ProducerMessage, error) {
	return nil, e.err
}

func (e logsErrorMarshaler) Encoding() string {
	panic("implement me")
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

func TestLoadEncodingExtension_logs(t *testing.T) {
	extension, err := loadEncodingExtension[plog.Marshaler](&testComponentHost{}, "logs_encoding")
	require.NoError(t, err)
	require.NotNil(t, extension)
}

func TestLoadEncodingExtension_notfound_error(t *testing.T) {
	extension, err := loadEncodingExtension[plog.Marshaler](&testComponentHost{}, "logs_notfound")
	require.Error(t, err)
	require.Nil(t, extension)
}

func TestLoadEncodingExtension_nomarshaler_error(t *testing.T) {
	extension, err := loadEncodingExtension[plog.Marshaler](&testComponentHost{}, "logs_nomarshaler")
	require.Error(t, err)
	require.Nil(t, extension)
}

type testComponentHost struct{}

func (h *testComponentHost) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.MustNewID("logs_encoding"):    &nopComponent{},
		component.MustNewID("logs_nomarshaler"): &nopNoMarshalerComponent{},
		component.MustNewID("metrics_encoding"): &nopComponent{},
		component.MustNewID("traces_encoding"):  &nopComponent{},
	}
}

type nopComponent struct{}

func (c *nopComponent) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *nopComponent) Shutdown(_ context.Context) error {
	return nil
}

func (c *nopComponent) MarshalLogs(_ plog.Logs) ([]byte, error) {
	return []byte{}, nil
}

func (c *nopComponent) MarshalMetrics(_ pmetric.Metrics) ([]byte, error) {
	return []byte{}, nil
}

func (c *nopComponent) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return []byte{}, nil
}

type nopNoMarshalerComponent struct{}

func (c *nopNoMarshalerComponent) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (c *nopNoMarshalerComponent) Shutdown(_ context.Context) error {
	return nil
}
