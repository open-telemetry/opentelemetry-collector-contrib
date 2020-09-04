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
		value    float64  = math.Pi
		tags     []string = []string{"tool:opentelemetry", "version:0.1.0"}
		rate     float64  = 1
	)

	metric := NewGauge(hostname, value, tags)

	assert.Equal(t, hostname, metric.GetHost())
	assert.Equal(t, Gauge, metric.GetType())
	assert.Equal(t, value, metric.GetValue())
	assert.Equal(t, tags, metric.GetTags())
	assert.Equal(t, rate, metric.GetRate())
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

	intValue := &v1.Point{
		Timestamp: metricstest.Timestamp(ts),
		Value:     &v1.Point_Int64Value{Int64Value: 17},
	}

	md := NewMetricsData([]*v1.Metric{
		metricstest.Gauge("gauge.float64.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.Double(ts, math.Pi))),
		metricstest.Cumulative("cumulative.float64.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.Double(ts, math.Pi))),

		metricstest.GaugeInt("gauge.int64.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], intValue)),
		metricstest.CumulativeInt("cumulative.int64.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], intValue)),
	})

	metrics, droppedTimeSeries := MapMetrics(mockExporter, md)

	assert.Equal(t, 0, droppedTimeSeries)
	assert.Equal(t,
		map[string][]MetricValue{
			"cumulative.float64.test": {{
				hostname:   "unknown",
				metricType: Gauge,
				value:      math.Pi,
				rate:       1,
				tags:       testTags[:],
			}},
			"gauge.float64.test": {{
				hostname:   "unknown",
				metricType: Gauge,
				value:      math.Pi,
				rate:       1,
				tags:       testTags[:],
			}},
			"gauge.int64.test": {{
				hostname:   "unknown",
				metricType: Gauge,
				value:      17,
				rate:       1,
				tags:       testTags[:],
			}},
			"cumulative.int64.test": {{
				hostname:   "unknown",
				metricType: Gauge,
				value:      17,
				rate:       1,
				tags:       testTags[:],
			}},
		},
		metrics,
	)

}

func TestMapDistributionMetric(t *testing.T) {
	ts := time.Now()
	md := NewMetricsData([]*v1.Metric{
		metricstest.GaugeDist("dist.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.DistPt(ts, []float64{0.1, 0.2}, []int64{100, 200}))),
	})

	metrics, _ := MapMetrics(
		&MockMetricsExporter{cfg: &Config{Metrics: MetricsConfig{Buckets: true}}},
		md,
	)

	assert.Equal(t, map[string][]MetricValue{
		"dist.test.count": {{
			hostname:   "unknown",
			metricType: Gauge,
			value:      300,
			rate:       1,
			tags:       testTags[:],
		}},
		"dist.test.sum": {{
			hostname:   "unknown",
			metricType: Gauge,
			// The sum value is approximated by the metricstest
			// package using lower bounds and counts:
			// sum = ∑ bound(i-1) * count(i)
			//     = 0*100 + 0.1*200
			//     = 20
			value: 20,
			rate:  1,
			tags:  testTags[:],
		}},
		"dist.test.squared_dev_sum": {{
			hostname:   "unknown",
			metricType: Gauge,
			// The sum of squared deviations is set to 0 by the
			// metricstest package
			value: 0,
			rate:  1,
			tags:  testTags[:],
		}},
		"dist.test.count_per_bucket": {
			{
				hostname:   "unknown",
				metricType: Gauge,
				value:      100,
				rate:       1,
				tags:       append(testTags[:], "bucket_idx:0"),
			},

			{
				hostname:   "unknown",
				metricType: Gauge,
				value:      200,
				rate:       1,
				tags:       append(testTags[:], "bucket_idx:1"),
			},
		},
	},
		metrics,
	)
}

func TestMapSummaryMetric(t *testing.T) {
	ts := time.Now()

	summaryPoint := metricstest.Timeseries(
		ts,
		testValues[:],
		metricstest.SummPt(ts, 5, 23, []float64{0, 50.1, 95, 100}, []float64{1, 22, 100, 300}),
	)

	md := NewMetricsData([]*v1.Metric{
		metricstest.Summary("summary.test", testKeys[:], summaryPoint),
	})

	metrics, _ := MapMetrics(
		// Enable percentiles for test
		&MockMetricsExporter{cfg: &Config{Metrics: MetricsConfig{Percentiles: true}}},
		md,
	)

	assert.Equal(t, map[string][]MetricValue{
		"summary.test.sum": {{
			hostname:   "unknown",
			metricType: Gauge,
			value:      23,
			rate:       1,
			tags:       testTags[:],
		}},
		"summary.test.count": {{
			hostname:   "unknown",
			metricType: Gauge,
			value:      5,
			rate:       1,
			tags:       testTags[:],
		}},
		"summary.test.p50": {{
			hostname:   "unknown",
			metricType: Gauge,
			value:      22,
			rate:       1,
			tags:       testTags[:],
		}},
		"summary.test.p95": {{
			hostname:   "unknown",
			metricType: Gauge,
			value:      100,
			rate:       1,
			tags:       testTags[:],
		}},
		"summary.test.min": {{
			hostname:   "unknown",
			metricType: Gauge,
			value:      1,
			rate:       1,
			tags:       testTags[:],
		}},
		"summary.test.max": {{
			hostname:   "unknown",
			metricType: Gauge,
			value:      300,
			rate:       1,
			tags:       testTags[:],
		}},
	},
		metrics,
	)
}

func TestMapInvalid(t *testing.T) {
	ts := time.Now()
	md := NewMetricsData([]*v1.Metric{{
		MetricDescriptor: &v1.MetricDescriptor{
			Type: v1.MetricDescriptor_UNSPECIFIED,
		},
		Timeseries: []*v1.TimeSeries{metricstest.Timeseries(
			ts, []string{}, metricstest.Double(ts, 0.0))},
	}})

	metrics, dropped := MapMetrics(mockExporter, md)

	assert.Equal(t, dropped, 1)
	assert.Equal(t, metrics, map[string][]MetricValue{})
}
