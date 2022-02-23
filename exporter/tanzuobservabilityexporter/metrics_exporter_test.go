// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tanzuobservabilityexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestPushMetricsData(t *testing.T) {
	assert.NoError(t, verifyPushMetricsData(t, false))
}

func TestPushMetricsDataErrorOnSend(t *testing.T) {
	assert.Error(t, verifyPushMetricsData(t, true))
}

func verifyPushMetricsData(t *testing.T, errorOnSend bool) error {
	metric := newMetric("test.metric", pdata.MetricDataTypeGauge)
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
	sender *mockMetricSender) (component.MetricsExporter, error) {
	cfg := createDefaultConfig()
	creator := func(
		hostName string, port int, settings component.TelemetrySettings, otelVersion string) (*metricsConsumer, error) {
		return newMetricsConsumer(
			[]typedMetricConsumer{
				newGaugeConsumer(sender, settings),
			},
			sender,
			false,
		), nil
	}

	exp, err := newMetricsExporter(componenttest.NewNopExporterCreateSettings(), cfg, creator)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		cfg,
		componenttest.NewNopExporterCreateSettings(),
		exp.pushMetricsData,
		exporterhelper.WithShutdown(exp.shutdown),
	)
}

func consumeMetrics(metrics pdata.Metrics, sender *mockMetricSender) error {
	ctx := context.Background()
	mockOTelMetricsExporter, err := createMockMetricsExporter(sender)
	if err != nil {
		return err
	}
	defer mockOTelMetricsExporter.Shutdown(ctx)
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
