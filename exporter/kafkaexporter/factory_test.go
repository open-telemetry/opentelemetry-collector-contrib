// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkaexporter

import (
	"context"
	"reflect"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
	assert.Equal(t, []string{defaultBroker}, cfg.Brokers)
	assert.Equal(t, "", cfg.Topic)
}

func TestCreateAllExporter(t *testing.T) {
	cfg0 := createDefaultConfig().(*Config)
	cfg1 := createDefaultConfig().(*Config)
	cfg2 := createDefaultConfig().(*Config)

	cfg0.Brokers = []string{"invalid:9092"}
	cfg1.Brokers = []string{"invalid:9092"}
	cfg2.Brokers = []string{"invalid:9092"}

	cfg0.ProtocolVersion = "2.0.0"
	cfg1.ProtocolVersion = "2.0.0"
	cfg2.ProtocolVersion = "2.0.0"

	// this disables contacting the broker so we can successfully create the exporter
	cfg0.Metadata.Full = false
	cfg1.Metadata.Full = false
	cfg2.Metadata.Full = false

	cfgClone := *cfg0 // Clone the config

	f := kafkaExporterFactory{tracesMarshalers: tracesMarshalers(), metricsMarshalers: metricsMarshalers(), logsMarshalers: logsMarshalers()}
	r0, err := f.createTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg0)
	require.NoError(t, err)
	r1, err := f.createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg1)
	require.NoError(t, err)
	r2, err := f.createLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg2)
	require.NoError(t, err)

	// createTracesExporter should not mutate values
	assert.True(t, reflect.DeepEqual(*cfg0, cfgClone), "config should not mutate")
	assert.True(t, reflect.DeepEqual(*cfg1, cfgClone), "config should not mutate")
	assert.True(t, reflect.DeepEqual(*cfg2, cfgClone), "config should not mutate")
	assert.NotNil(t, r0)
	assert.NotNil(t, r1)
	assert.NotNil(t, r2)
}

func TestCreateTracesExporter(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	// this disables contacting the broker so we can successfully create the exporter
	cfg.Metadata.Full = false
	f := kafkaExporterFactory{tracesMarshalers: tracesMarshalers()}
	r, err := f.createTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestCreateMetricsExport(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	// this disables contacting the broker so we can successfully create the exporter
	cfg.Metadata.Full = false
	mf := kafkaExporterFactory{metricsMarshalers: metricsMarshalers()}
	mr, err := mf.createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, mr)
}

func TestCreateLogsExport(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	// this disables contacting the broker so we can successfully create the exporter
	cfg.Metadata.Full = false
	mf := kafkaExporterFactory{logsMarshalers: logsMarshalers()}
	mr, err := mf.createLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, mr)
}

func TestCreateTracesExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	f := kafkaExporterFactory{tracesMarshalers: tracesMarshalers()}
	r, err := f.createTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	// no available broker
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	mf := kafkaExporterFactory{metricsMarshalers: metricsMarshalers()}
	mr, err := mf.createMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	require.Error(t, err)
	assert.Nil(t, mr)
}

func TestCreateLogsExporter_err(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	mf := kafkaExporterFactory{logsMarshalers: logsMarshalers()}
	mr, err := mf.createLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
	require.Error(t, err)
	assert.Nil(t, mr)
}

func setupExporterFactoryForMarshalerTests(encoding string) (*Config, component.ExporterFactory, *customTracesMarshaler, *customMetricsMarshaler, *customLogsMarshaler) {
	cfg := createDefaultConfig().(*Config)
	cfg.Encoding = encoding
	// disable contacting broker
	cfg.Metadata.Full = false
	cfg.RetrySettings = exporterhelper.RetrySettings{
		Enabled: false,
	}
	cfg.QueueSettings = exporterhelper.QueueSettings{
		Enabled: false,
	}
	traceMarshaler := &customTracesMarshaler{}
	metricsMarshaler := &customMetricsMarshaler{}
	logMarshaler := &customLogsMarshaler{}
	f := NewFactory(WithTracesMarshalers(traceMarshaler), WithMetricsMarshalers(metricsMarshaler), WithLogsMarshalers(logMarshaler))

	return cfg, f, traceMarshaler, metricsMarshaler, logMarshaler
}

func TestWithTracesMarshalers(t *testing.T) {

	span := ptrace.NewSpan()
	span.SetName("testSpan")

	scopeSpans := ptrace.NewScopeSpans()
	scopeSpans.Spans().AppendEmpty().CopyTo(span)

	resourceSpans := ptrace.NewResourceSpans()
	resourceSpans.ScopeSpans().AppendEmpty().CopyTo(scopeSpans)

	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty().CopyTo(resourceSpans)

	t.Run("custom_encoding", func(t *testing.T) {
		cfg, exporterFactory, customTraceMarshaler, _, _ := setupExporterFactoryForMarshalerTests("custom")

		exporter, err := exporterFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeTraces(context.TODO(), traces)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		require.NotNil(t, exporter)
		require.Len(t, customTraceMarshaler.traces, 1)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg, exporterFactory, customTraceMarshaler, _, _ := setupExporterFactoryForMarshalerTests(defaultEncoding)

		exporter, err := exporterFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeTraces(context.TODO(), traces)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		assert.NotNil(t, exporter)
		// custom marshaler should have not been called for defaultEncoding
		require.Len(t, customTraceMarshaler.traces, 0)
	})
}

type customTracesMarshaler struct {
	traces []ptrace.Traces
}

var _ TracesMarshaler = (*customTracesMarshaler)(nil)

func (c *customTracesMarshaler) Marshal(td ptrace.Traces, topic string) ([]*sarama.ProducerMessage, error) {
	c.traces = append(c.traces, td)
	return make([]*sarama.ProducerMessage, 0), nil
}

func (c *customTracesMarshaler) Encoding() string {
	return "custom"
}

func TestWithLogsMarshalers(t *testing.T) {

	logRecord := plog.NewLogRecord()
	logRecord.Body().SetStringVal("test")

	scopeLogs := plog.NewScopeLogs()
	scopeLogs.LogRecords().AppendEmpty().CopyTo(logRecord)

	resourceLogs := plog.NewResourceLogs()
	resourceLogs.ScopeLogs().AppendEmpty().CopyTo(scopeLogs)

	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().CopyTo(resourceLogs)

	t.Run("custom_encoding", func(t *testing.T) {
		cfg, exporterFactory, _, _, customLogsMarshaler := setupExporterFactoryForMarshalerTests("custom")

		exporter, err := exporterFactory.CreateLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeLogs(context.TODO(), logs)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		require.NotNil(t, exporter)
		require.Len(t, customLogsMarshaler.logs, 1)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg, exporterFactory, _, _, customLogsMarshaler := setupExporterFactoryForMarshalerTests(defaultEncoding)

		exporter, err := exporterFactory.CreateLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeLogs(context.TODO(), logs)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		assert.NotNil(t, exporter)
		// custom marshaler should have not been called for defaultEncoding
		require.Len(t, customLogsMarshaler.logs, 0)
	})

}

type customLogsMarshaler struct {
	logs []plog.Logs
}

var _ LogsMarshaler = (*customLogsMarshaler)(nil)

func (c *customLogsMarshaler) Marshal(logs plog.Logs, topic string) ([]*sarama.ProducerMessage, error) {
	c.logs = append(c.logs, logs)
	return make([]*sarama.ProducerMessage, 0), nil
}

func (c *customLogsMarshaler) Encoding() string {
	return "custom"
}

func TestWithMetricsMarshalers(t *testing.T) {

	metric := pmetric.NewMetric()
	metric.SetName("test")

	scopeMetrics := pmetric.NewScopeMetrics()
	scopeMetrics.Metrics().AppendEmpty().CopyTo(metric)

	resourceMetrics := pmetric.NewResourceMetrics()
	resourceMetrics.ScopeMetrics().AppendEmpty().CopyTo(scopeMetrics)

	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty().CopyTo(resourceMetrics)

	t.Run("custom_encoding", func(t *testing.T) {
		cfg, exporterFactory, _, customMetricsMarshaler, _ := setupExporterFactoryForMarshalerTests("custom")

		exporter, err := exporterFactory.CreateMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeMetrics(context.TODO(), metrics)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		require.NotNil(t, exporter)
		require.Len(t, customMetricsMarshaler.metrics, 1)
	})
	t.Run("default_encoding", func(t *testing.T) {
		cfg, exporterFactory, _, customMetricsMarshaler, _ := setupExporterFactoryForMarshalerTests(defaultEncoding)

		exporter, err := exporterFactory.CreateMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeMetrics(context.TODO(), metrics)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		assert.NotNil(t, exporter)
		// custom marshaler should have not been called for defaultEncoding
		require.Len(t, customMetricsMarshaler.metrics, 0)
	})

}

type customMetricsMarshaler struct {
	metrics []pmetric.Metrics
}

var _ MetricsMarshaler = (*customMetricsMarshaler)(nil)

func (c *customMetricsMarshaler) Marshal(metrics pmetric.Metrics, topic string) ([]*sarama.ProducerMessage, error) {
	c.metrics = append(c.metrics, metrics)
	return make([]*sarama.ProducerMessage, 0), nil
}

func (c *customMetricsMarshaler) Encoding() string {
	return "custom"
}
