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
	"fmt"
	"reflect"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/Shopify/sarama/mocks"
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

func TestWithMarshalers(t *testing.T) {

	customMarshalerEncoding := "custom"

	// For simplicity, all the custom marshalers will produce the same message
	customMarshalMessage := &sarama.ProducerMessage{
		Topic: "any_topic",
		Key:   sarama.StringEncoder("key that could only come from the custom marshaller"),
		Value: sarama.StringEncoder("value"),
	}

	// sets up mock to either expect the custom message or not
	setupProducerMock := func(expectCustom bool) sarama.SyncProducer {
		producer := mocks.NewSyncProducer(t, nil)
		producer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(func(pm *sarama.ProducerMessage) error {
			isCustom := pm != nil &&
				pm.Topic == customMarshalMessage.Topic &&
				pm.Key == customMarshalMessage.Key &&
				pm.Value == customMarshalMessage.Value
			if expectCustom {
				if !isCustom {
					return fmt.Errorf("marshaled message was not custom")
				}
			} else {
				if isCustom {
					return fmt.Errorf("marshaled message was custom and expecting default")
				}
			}
			return nil
		})
		return producer
	}

	// config is consistent except encoding to use
	setupConfig := func(encoding string) *Config {
		cfg := createDefaultConfig().(*Config)
		cfg.Encoding = encoding
		cfg.QueueSettings = exporterhelper.QueueSettings{
			Enabled: false,
		}
		cfg.RetrySettings = exporterhelper.RetrySettings{
			Enabled: false,
		}
		return cfg
	}

	// all the marshaler tests are setup the same
	setupFactory := func(producer sarama.SyncProducer) component.ExporterFactory {
		f := NewFactory(
			WithProducerFactory(func(config Config) (sarama.SyncProducer, error) { return producer, nil }),
			WithTracesMarshalers(&mockTraceMarshaler{
				MarshalFunc: func(traces ptrace.Traces, topic string) ([]*sarama.ProducerMessage, error) {
					return []*sarama.ProducerMessage{
						customMarshalMessage,
					}, nil
				},
				EncodingValue: customMarshalerEncoding,
			}),
			WithMetricsMarshalers(&mockMetricsMarshaler{
				MarshalFunc: func(metrics pmetric.Metrics, topic string) ([]*sarama.ProducerMessage, error) {
					return []*sarama.ProducerMessage{
						customMarshalMessage,
					}, nil
				},
				EncodingValue: customMarshalerEncoding,
			}),
			WithLogsMarshalers(&mockLogMarshaler{
				MarshalFunc: func(logs plog.Logs, topic string) ([]*sarama.ProducerMessage, error) {
					return []*sarama.ProducerMessage{
						customMarshalMessage,
					}, nil
				},
				EncodingValue: customMarshalerEncoding,
			}),
		)

		return f
	}

	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetName("testSpan")

	t.Run("traces_custom_encoding", func(t *testing.T) {
		cfg := setupConfig(customMarshalerEncoding)
		producer := setupProducerMock(true)
		exporterFactory := setupFactory(producer)

		exporter, err := exporterFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeTraces(context.TODO(), traces)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		require.NotNil(t, exporter)
	})
	t.Run("traces_default_encoding", func(t *testing.T) {
		cfg := setupConfig(defaultEncoding)
		producer := setupProducerMock(false)
		exporterFactory := setupFactory(producer)

		exporter, err := exporterFactory.CreateTracesExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeTraces(context.TODO(), traces)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		assert.NotNil(t, exporter)
	})

	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("test")
	t.Run("metrics_custom_encoding", func(t *testing.T) {
		cfg := setupConfig(customMarshalerEncoding)
		producer := setupProducerMock(true)
		exporterFactory := setupFactory(producer)

		exporter, err := exporterFactory.CreateMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeMetrics(context.TODO(), metrics)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		require.NotNil(t, exporter)
	})
	t.Run("metrics_default_encoding", func(t *testing.T) {
		cfg := setupConfig(defaultEncoding)
		producer := setupProducerMock(false)
		exporterFactory := setupFactory(producer)

		exporter, err := exporterFactory.CreateMetricsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeMetrics(context.TODO(), metrics)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		assert.NotNil(t, exporter)
	})

	logs := plog.NewLogs()
	logs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStringVal("test")
	t.Run("logs_custom_encoding", func(t *testing.T) {
		cfg := setupConfig(customMarshalerEncoding)
		producer := setupProducerMock(true)
		exporterFactory := setupFactory(producer)

		exporter, err := exporterFactory.CreateLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeLogs(context.TODO(), logs)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		require.NotNil(t, exporter)
	})
	t.Run("logs_default_encoding", func(t *testing.T) {
		cfg := setupConfig(defaultEncoding)
		producer := setupProducerMock(false)
		exporterFactory := setupFactory(producer)

		exporter, err := exporterFactory.CreateLogsExporter(context.Background(), componenttest.NewNopExporterCreateSettings(), cfg)
		consumeErr := exporter.ConsumeLogs(context.TODO(), logs)
		require.NoError(t, err)
		require.NoError(t, consumeErr)
		assert.NotNil(t, exporter)
	})

}
