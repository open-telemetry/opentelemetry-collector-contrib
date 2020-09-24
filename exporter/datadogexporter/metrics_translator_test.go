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
	"math"
	"testing"
	"time"

	v1agent "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	metricstest "go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.uber.org/zap"
	"gopkg.in/zorkian/go-datadog-api.v2"
)

func TestMetricValue(t *testing.T) {
	var (
		hostname string   = "unknown"
		name     string   = "name"
		value    float64  = math.Pi
		ts       int32    = int32(time.Now().Unix())
		tags     []string = []string{"tool:opentelemetry", "version:0.1.0"}
	)

	metric := NewGauge(hostname, name, ts, value, tags)

	assert.Equal(t, hostname, metric.GetHost())
	assert.Equal(t, Gauge, metric.GetType())
	assert.Equal(t, tags, metric.Tags)
}

var (
	testHost   = "unknown"
	testKeys   = [...]string{"key1", "key2", "key3"}
	testValues = [...]string{"val1", "val2", ""}
	testTags   = [...]string{"key1:val1", "key2:val2", "key3:n/a"}
	logger     = zap.NewNop()
)

func NewMetricsData(metrics []*v1.Metric) []consumerdata.MetricsData {
	return []consumerdata.MetricsData{{
		Node: &v1agent.Node{
			Identifier: &v1agent.ProcessIdentifier{
				HostName: "unknown",
			},
		},
		Resource: nil,
		Metrics:  metrics,
	}}
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

	series, droppedTimeSeries := MapMetrics(logger, MetricsConfig{}, md)

	assert.Equal(t, 0, droppedTimeSeries)
	assert.ElementsMatch(t,
		[]datadog.Metric{
			NewGauge(
				testHost,
				"gauge.float64.test",
				int32(ts.Unix()),
				math.Pi,
				testTags[:],
			),
			NewGauge(
				testHost,
				"cumulative.float64.test",
				int32(ts.Unix()),
				math.Pi,
				testTags[:],
			),
			NewGauge(
				testHost,
				"gauge.int64.test",
				int32(ts.Unix()),
				17,
				testTags[:],
			),
			NewGauge(
				testHost,
				"cumulative.int64.test",
				int32(ts.Unix()),
				17,
				testTags[:],
			),
		},
		series.metrics,
	)

}

func TestMapDistributionMetric(t *testing.T) {
	ts := time.Now()
	md := NewMetricsData([]*v1.Metric{
		metricstest.GaugeDist("dist.test", testKeys[:],
			metricstest.Timeseries(ts, testValues[:], metricstest.DistPt(ts, []float64{0.1, 0.2}, []int64{100, 200}))),
	})

	series, _ := MapMetrics(
		logger,
		MetricsConfig{Buckets: true},
		md,
	)

	assert.ElementsMatch(t, []datadog.Metric{
		NewGauge(
			testHost,
			"dist.test.count",
			int32(ts.Unix()),
			300,
			testTags[:],
		),
		// The sum value is approximated by the metricstest
		// package using lower bounds and counts:
		// sum = ∑ bound(i-1) * count(i)
		//     = 0*100 + 0.1*200
		//     = 20
		NewGauge(
			testHost,
			"dist.test.sum",
			int32(ts.Unix()),
			20,
			testTags[:],
		),
		// The sum of squared deviations is set to 0 by the
		// metricstest package
		NewGauge(
			testHost,
			"dist.test.squared_dev_sum",
			int32(ts.Unix()),
			0,
			testTags[:],
		),
		NewGauge(
			testHost,
			"dist.test.count_per_bucket",
			int32(ts.Unix()),
			100,
			append(testTags[:], "bucket_idx:0"),
		),
		NewGauge(
			testHost,
			"dist.test.count_per_bucket",
			int32(ts.Unix()),
			200,
			append(testTags[:], "bucket_idx:1"),
		),
	},
		series.metrics,
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

	series, _ := MapMetrics(
		logger,
		// Enable percentiles for test
		MetricsConfig{Percentiles: true},
		md,
	)

	assert.ElementsMatch(t, []datadog.Metric{
		NewGauge(
			testHost,
			"summary.test.count",
			int32(ts.Unix()),
			5,
			testTags[:],
		),
		NewGauge(
			testHost,
			"summary.test.sum",
			int32(ts.Unix()),
			23,
			testTags[:],
		),
		NewGauge(
			testHost,
			"summary.test.min",
			int32(ts.Unix()),
			1,
			testTags[:],
		),
		NewGauge(
			testHost,
			"summary.test.p50",
			int32(ts.Unix()),
			22,
			testTags[:],
		),
		NewGauge(
			testHost,
			"summary.test.p95",
			int32(ts.Unix()),
			100,
			testTags[:],
		),
		NewGauge(
			testHost,
			"summary.test.max",
			int32(ts.Unix()),
			300,
			testTags[:],
		),
	},
		series.metrics,
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

	metrics, droppedTimeSeries := MapMetrics(logger, MetricsConfig{}, md)

	assert.Equal(t, droppedTimeSeries, 1)
	assert.Equal(t, metrics, Series{})
}
