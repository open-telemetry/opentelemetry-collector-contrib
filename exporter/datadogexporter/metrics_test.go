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

package datadogexporter

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	v1agent "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/pdatautil"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	metricstest "go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.uber.org/zap"
)

type MockMetricsExporter struct {
	cfg *Config
}

func (m *MockMetricsExporter) PushMetricsData(ctx context.Context, md pdata.Metrics) (int, error) {
	return 0, fmt.Errorf("Mock metrics exporter")
}

func (m *MockMetricsExporter) GetLogger() *zap.Logger {
	return zap.NewNop()
}

func (m *MockMetricsExporter) GetConfig() *Config {
	return m.cfg
}

func (m *MockMetricsExporter) GetQueueSettings() exporterhelper.QueueSettings {
	return exporterhelper.CreateDefaultQueueSettings()
}

func (m *MockMetricsExporter) GetRetrySettings() exporterhelper.RetrySettings {
	return exporterhelper.CreateDefaultRetrySettings()

}

func TestMetricValue(t *testing.T) {
	var (
		hostname string   = "unknown"
		name     string   = "metric.name"
		value    float64  = math.Pi
		tags     []string = []string{"tool:opentelemetry", "version:0.1.0"}
		rate     float64  = 1
	)

	metric, err := NewMetric(
		hostname,
		name,
		Gauge,
		value,
		tags,
		rate,
	)

	assert.Nil(t, err)
	assert.NotNil(t, metric)

	assert.Equal(t, hostname, metric.GetHost())
	assert.Equal(t, name, metric.GetName())
	assert.Equal(t, Gauge, metric.GetType())
	assert.Equal(t, value, metric.GetValue())
	assert.Equal(t, tags, metric.GetTags())
	assert.Equal(t, rate, metric.GetRate())

	// Fail when using incorrect type
	nilMetric, err := NewMetric(
		hostname,
		name,
		Count,
		value,
		tags,
		rate,
	)

	require.Error(t, err)
	assert.Nil(t, nilMetric)

}

var (
	mockExporter = &MockMetricsExporter{cfg: &Config{}}
	testKeys     = [...]string{"key1", "key2", "key3"}
	testValues   = [...]string{"val1", "val2", ""}
	testTags     = [...]string{"key1:val1", "key2:val2", "key3:n/a"}
)

func NewMetricsData(metrics []*v1.Metric) pdata.Metrics {
	return pdatautil.MetricsFromMetricsData([]consumerdata.MetricsData{
		{
			Node: &v1agent.Node{
				Identifier: &v1agent.ProcessIdentifier{
					HostName: "unknown",
				},
			},
			Resource: nil,
			Metrics:  metrics,
		},
	})
}

func TestMapNumericMetric(t *testing.T) {
	ts := time.Now()

	md := NewMetricsData([]*v1.Metric{
		metricstest.Gauge("gauge.float64.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.Double(ts, math.Pi))),
		metricstest.Cumulative("cumulative.float64.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.Double(ts, math.Pi))),
	})

	metrics, droppedTimeSeries, err := MapMetrics(mockExporter, md)

	assert.Equal(t, 0, droppedTimeSeries)
	assert.Nil(t, err)
	assert.Equal(t,
		map[string][]*Metric{
			"cumulative.float64.test": {
				{
					hostname:   "unknown",
					name:       "cumulative.float64.test",
					metricType: Gauge,
					fvalue:     math.Pi,
					rate:       1,
					tags:       testTags[:],
				},
			},
			"gauge.float64.test": {
				{
					hostname:   "unknown",
					name:       "gauge.float64.test",
					metricType: Gauge,
					fvalue:     math.Pi,
					rate:       1,
					tags:       testTags[:],
				},
			},
		},
		metrics,
	)

}

func TestMapDistributionMetric(t *testing.T) {
	ts := time.Now()
	md := NewMetricsData([]*v1.Metric{
		metricstest.GaugeDist("dist.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.DistPt(ts, []float64{}, []int64{}))),
		metricstest.CumulativeDist("cumulative.dist.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.DistPt(ts, []float64{}, []int64{}))),
	})

	metrics, _, err := MapMetrics(mockExporter, md)

	// Right now they are silently ignored
	assert.Nil(t, err)
	assert.Equal(t, map[string][]*Metric{}, metrics)
}

func TestMapSummaryMetric(t *testing.T) {
	ts := time.Now()

	md := NewMetricsData([]*v1.Metric{
		metricstest.Summary("summary.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.SummPt(ts, 2, 10, []float64{}, []float64{}))),
	})

	metrics, _, err := MapMetrics(mockExporter, md)

	// Right now they are silently ignored
	assert.Nil(t, err)
	assert.Equal(t, map[string][]*Metric{}, metrics)
}
