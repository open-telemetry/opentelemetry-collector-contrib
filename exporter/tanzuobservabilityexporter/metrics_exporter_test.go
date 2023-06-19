// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter

import (
	"context"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestPushMetricsData(t *testing.T) {
	assert.NoError(t, verifyPushMetricsData(t, false))
}

func TestPushMetricsDataErrorOnSend(t *testing.T) {
	assert.Error(t, verifyPushMetricsData(t, true))
}

func verifyPushMetricsData(t *testing.T, errorOnSend bool) error {
	metric := newMetric("test.metric", pmetric.MetricTypeGauge)
	dataPoints := metric.Gauge().DataPoints()
	dataPoints.EnsureCapacity(1)
	addDataPoint(
		7,
		1631205001,
		map[string]interface{}{
			"env":    "prod",
			"bucket": 73,
		},
		dataPoints,
	)
	metrics := constructMetrics(metric)
	sender := &mockMetricSender{errorOnSend: errorOnSend}
	result := consumeMetrics(metrics, sender)
	assert.Equal(t, 1, sender.numFlushCalls)
	assert.Equal(t, 1, sender.numCloseCalls)
	assert.Equal(t, 1, sender.numSendMetricCalls)
	return result
}

func createMockMetricsExporter(
	sender *mockMetricSender) (exporter.Metrics, error) {
	exporterConfig := createDefaultConfig()
	tobsConfig := exporterConfig.(*Config)
	tobsConfig.Metrics.Endpoint = "http://localhost:2878"
	creator := func(
		metricsConfig MetricsConfig, settings component.TelemetrySettings, otelVersion string) (*metricsConsumer, error) {
		return newMetricsConsumer(
			[]typedMetricConsumer{
				newGaugeConsumer(sender, settings),
			},
			sender,
			false,
			tobsConfig.Metrics,
		), nil
	}

	exp, err := newMetricsExporter(exportertest.NewNopCreateSettings(), exporterConfig, creator)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		exporterConfig,
		exp.pushMetricsData,
		exporterhelper.WithShutdown(exp.shutdown),
	)
}

func consumeMetrics(metrics pmetric.Metrics, sender *mockMetricSender) error {
	ctx := context.Background()
	mockOTelMetricsExporter, err := createMockMetricsExporter(sender)
	if err != nil {
		return err
	}
	defer func() {
		if err := mockOTelMetricsExporter.Shutdown(ctx); err != nil {
			log.Fatalln(err)
		}
	}()
	return mockOTelMetricsExporter.ConsumeMetrics(ctx, metrics)
}

type mockMetricSender struct {
	errorOnSend        bool
	numFlushCalls      int
	numCloseCalls      int
	numSendMetricCalls int
}

func (m *mockMetricSender) SendMetric(
	_ string, _ float64, _ int64, _ string, _ map[string]string) error {
	m.numSendMetricCalls++
	if m.errorOnSend {
		return errors.New("error sending")
	}
	return nil
}

func (m *mockMetricSender) Flush() error {
	m.numFlushCalls++
	return nil
}

func (m *mockMetricSender) Close() { m.numCloseCalls++ }
