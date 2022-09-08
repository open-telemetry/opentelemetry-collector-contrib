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

package internal

import (
	"regexp"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var (
	startTsNanos             = pcommon.Timestamp(startTs * 1e6)
	startTsPlusIntervalNanos = pcommon.Timestamp((startTs + interval) * 1e6)
)

func TestStartTimeMetricMatch(t *testing.T) {
	matchBuilderStartTime := 123.456
	tests := []struct {
		name                     string
		inputs                   []*testScrapedPage
		expectedBuilderStartTime float64
		startTimeMetricRegex     *regexp.Regexp
	}{
		{
			name: "regexp_match",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("example_process_start_time_seconds", matchBuilderStartTime, "foo", "bar"),
						createDataPoint("process_start_time_seconds", matchBuilderStartTime+1, "foo", "bar"),
					},
				},
			},
			expectedBuilderStartTime: matchBuilderStartTime,
			startTimeMetricRegex:     regexp.MustCompile("^.*_process_start_time_seconds$"),
		},
		{
			name: "match_default_start_time_metric",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("example_process_start_time_seconds", matchBuilderStartTime, "foo", "bar"),
						createDataPoint("process_start_time_seconds", matchBuilderStartTime+1, "foo", "bar"),
					},
				},
			},
			expectedBuilderStartTime: matchBuilderStartTime + 1,
		},
		{
			name: "regexp_nomatch",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("subprocess_start_time_seconds", matchBuilderStartTime+2, "foo", "bar"),
					},
				},
			},
			expectedBuilderStartTime: 0,
			startTimeMetricRegex:     regexp.MustCompile("^.+_process_start_time_seconds$"),
		},
		{
			name: "nomatch_default_start_time_metric",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("subprocess_start_time_seconds", matchBuilderStartTime+2, "foo", "bar"),
					},
				},
			},
			expectedBuilderStartTime: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := testMetadataStore(testMetadata)
			st := startTs
			for _, page := range tt.inputs {
				b := newMetricBuilder(mc, true, tt.startTimeMetricRegex, zap.NewNop())
				for _, pt := range page.pts {
					// set ts for testing
					pt.t = st
					assert.NoError(t, b.AddDataPoint(pt.lb, pt.t, pt.v))
				}
				_, err := b.getMetrics(pcommon.NewResource())
				assert.NoError(t, err)
				assert.EqualValues(t, b.startTime, tt.expectedBuilderStartTime)
				st += interval
			}
		})
	}
}

func TestMetricBuilderCounters(t *testing.T) {
	tests := []buildTestData{
		{
			name: "single-item",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("counter_test", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("counter_test")
				m0.SetDataType(pmetric.MetricDataTypeSum)
				sum := m0.Sum()
				sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)
				pt0 := sum.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "two-items",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("counter_test", 150, "foo", "bar"),
						createDataPoint("counter_test", 25, "foo", "other"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("counter_test")
				m0.SetDataType(pmetric.MetricDataTypeSum)
				sum := m0.Sum()
				sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)
				pt0 := sum.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(150.0)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				pt1 := sum.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(25.0)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().UpsertString("foo", "other")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "two-metrics",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("counter_test", 150, "foo", "bar"),
						createDataPoint("counter_test", 25, "foo", "other"),
						createDataPoint("counter_test2", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("counter_test")
				m0.SetDataType(pmetric.MetricDataTypeSum)
				sum0 := m0.Sum()
				sum0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				sum0.SetIsMonotonic(true)
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(150.0)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				pt1 := sum0.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(25.0)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().UpsertString("foo", "other")

				m1 := mL0.AppendEmpty()
				m1.SetName("counter_test2")
				m1.SetDataType(pmetric.MetricDataTypeSum)
				sum1 := m1.Sum()
				sum1.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				sum1.SetIsMonotonic(true)
				pt2 := sum1.DataPoints().AppendEmpty()
				pt2.SetDoubleVal(100.0)
				pt2.SetStartTimestamp(startTsNanos)
				pt2.SetTimestamp(startTsNanos)
				pt2.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "metrics-with-poor-names",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("poor_name_count", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("poor_name_count")
				m0.SetDataType(pmetric.MetricDataTypeSum)
				sum := m0.Sum()
				sum.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)
				pt0 := sum.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderGauges(t *testing.T) {
	tests := []buildTestData{
		{
			name: "one-gauge",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 100, "foo", "bar"),
					},
				},
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 90, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				md1 := pmetric.NewMetrics()
				mL1 := md1.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(90.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(startTsPlusIntervalNanos)
				pt1.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0, md1}
			},
		},
		{
			name: "gauge-with-different-tags",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 100, "foo", "bar"),
						createDataPoint("gauge_test", 200, "bar", "foo"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				pt1 := gauge0.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(200.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().UpsertString("bar", "foo")

				return []pmetric.Metrics{md0}
			},
		},
		{
			// TODO: A decision need to be made. If we want to have the behavior which can generate different tag key
			//  sets because metrics come and go
			name: "gauge-comes-and-go-with-different-tagset",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 100, "foo", "bar"),
						createDataPoint("gauge_test", 200, "bar", "foo"),
					},
				},
				{
					pts: []*testDataPoint{
						createDataPoint("gauge_test", 20, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				pt1 := gauge0.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(200.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().UpsertString("bar", "foo")

				md1 := pmetric.NewMetrics()
				mL1 := md1.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt2 := gauge1.DataPoints().AppendEmpty()
				pt2.SetDoubleVal(20.0)
				pt2.SetStartTimestamp(0)
				pt2.SetTimestamp(startTsPlusIntervalNanos)
				pt2.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0, md1}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderUntyped(t *testing.T) {
	tests := []buildTestData{
		{
			name: "one-unknown",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("unknown_test", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("unknown_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "no-type-hint",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("something_not_exists", 100, "foo", "bar"),
						createDataPoint("theother_not_exists", 200, "foo", "bar"),
						createDataPoint("theother_not_exists", 300, "bar", "foo"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("something_not_exists")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				m1 := mL0.AppendEmpty()
				m1.SetName("theother_not_exists")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(200.0)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().UpsertString("foo", "bar")

				pt2 := gauge1.DataPoints().AppendEmpty()
				pt2.SetDoubleVal(300.0)
				pt2.SetTimestamp(startTsNanos)
				pt2.Attributes().UpsertString("bar", "foo")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "untype-metric-poor-names",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("some_count", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("some_count")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderHistogram(t *testing.T) {
	tests := []buildTestData{
		{
			name: "single item",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 8}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "multi-groups",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
						createDataPoint("hist_test_bucket", 1, "key2", "v2", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "key2", "v2", "le", "20"),
						createDataPoint("hist_test_bucket", 3, "key2", "v2", "le", "+inf"),
						createDataPoint("hist_test_sum", 50, "key2", "v2"),
						createDataPoint("hist_test_count", 3, "key2", "v2"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 8}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				pt1 := hist0.DataPoints().AppendEmpty()
				pt1.SetCount(3)
				pt1.SetSum(50)
				pt1.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt1.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1}))
				pt1.SetTimestamp(startTsNanos)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.Attributes().UpsertString("key2", "v2")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "multi-groups-and-families",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
						createDataPoint("hist_test_bucket", 1, "key2", "v2", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "key2", "v2", "le", "20"),
						createDataPoint("hist_test_bucket", 3, "key2", "v2", "le", "+inf"),
						createDataPoint("hist_test_sum", 50, "key2", "v2"),
						createDataPoint("hist_test_count", 3, "key2", "v2"),
						createDataPoint("hist_test2_bucket", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test2_bucket", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test2_bucket", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test2_sum", 50, "foo", "bar"),
						createDataPoint("hist_test2_count", 3, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 8}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				pt1 := hist0.DataPoints().AppendEmpty()
				pt1.SetCount(3)
				pt1.SetSum(50)
				pt1.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt1.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1}))
				pt1.SetTimestamp(startTsNanos)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.Attributes().UpsertString("key2", "v2")

				m1 := mL0.AppendEmpty()
				m1.SetName("hist_test2")
				m1.SetDataType(pmetric.MetricDataTypeHistogram)
				hist1 := m1.Histogram()
				hist1.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt2 := hist1.DataPoints().AppendEmpty()
				pt2.SetCount(3)
				pt2.SetSum(50)
				pt2.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt2.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1}))
				pt2.SetTimestamp(startTsNanos)
				pt2.SetStartTimestamp(startTsNanos)
				pt2.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "unordered-buckets",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_bucket", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(10)
				pt0.SetSum(99)
				pt0.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 8}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_count", 3, "foo", "bar"),
						createDataPoint("hist_test_sum", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(3)
				pt0.SetSum(100)
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{3}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket-noninf",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 3, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_count", 3, "foo", "bar"),
						createDataPoint("hist_test_sum", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(3)
				pt0.SetSum(100)
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{3}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "no-sum",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_count", 3, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(3)
				pt0.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "corrupted-no-buckets",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_sum", 99),
						createDataPoint("hist_test_count", 10),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "corrupted-no-count",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test_bucket", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test_bucket", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_bucket", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				return []pmetric.Metrics{md0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestMetricBuilderSummary(t *testing.T) {
	tests := []buildTestData{
		{
			name: "no-sum-and-count",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 5, "foo", "bar", "quantile", "1"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "no-count",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 1, "foo", "bar", "quantile", "0.5"),
						createDataPoint("summary_test", 2, "foo", "bar", "quantile", "0.75"),
						createDataPoint("summary_test", 5, "foo", "bar", "quantile", "1"),
						createDataPoint("summary_test_sum", 500, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "no-sum",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 1, "foo", "bar", "quantile", "0.5"),
						createDataPoint("summary_test", 2, "foo", "bar", "quantile", "0.75"),
						createDataPoint("summary_test", 5, "foo", "bar", "quantile", "1"),
						createDataPoint("summary_test_count", 500, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("summary_test")
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				sum0 := m0.Summary()
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.SetCount(500)
				pt0.SetSum(0.0)
				pt0.Attributes().UpsertString("foo", "bar")
				qvL := pt0.QuantileValues()
				q50 := qvL.AppendEmpty()
				q50.SetQuantile(.50)
				q50.SetValue(1.0)
				q75 := qvL.AppendEmpty()
				q75.SetQuantile(.75)
				q75.SetValue(2.0)
				q100 := qvL.AppendEmpty()
				q100.SetQuantile(1)
				q100.SetValue(5.0)

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "empty-quantiles",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test_sum", 100, "foo", "bar"),
						createDataPoint("summary_test_count", 500, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("summary_test")
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				sum0 := m0.Summary()
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(startTsNanos)
				pt0.SetTimestamp(startTsNanos)
				pt0.SetCount(500)
				pt0.SetSum(100.0)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.Metrics{md0}
			},
		},
		{
			name: "regular-summary",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("summary_test", 1, "foo", "bar", "quantile", "0.5"),
						createDataPoint("summary_test", 2, "foo", "bar", "quantile", "0.75"),
						createDataPoint("summary_test", 5, "foo", "bar", "quantile", "1"),
						createDataPoint("summary_test_sum", 100, "foo", "bar"),
						createDataPoint("summary_test_count", 500, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.Metrics {
				md0 := pmetric.NewMetrics()
				mL0 := md0.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				m0 := mL0.AppendEmpty()
				m0.SetName("summary_test")
				m0.SetDataType(pmetric.MetricDataTypeSummary)
				sum0 := m0.Summary()
				pt0 := sum0.DataPoints().AppendEmpty()
				pt0.SetStartTimestamp(startTsNanos)
				pt0.SetTimestamp(startTsNanos)
				pt0.SetCount(500)
				pt0.SetSum(100.0)
				pt0.Attributes().UpsertString("foo", "bar")
				qvL := pt0.QuantileValues()
				q50 := qvL.AppendEmpty()
				q50.SetQuantile(.50)
				q50.SetValue(1.0)
				q75 := qvL.AppendEmpty()
				q75.SetQuantile(.75)
				q75.SetValue(2.0)
				q100 := qvL.AppendEmpty()
				q100.SetQuantile(1)
				q100.SetValue(5.0)

				return []pmetric.Metrics{md0}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}

}

type buildTestData struct {
	name   string
	inputs []*testScrapedPage
	wants  func() []pmetric.Metrics
}

func (tt buildTestData) run(t *testing.T) {
	wants := tt.wants()
	assert.EqualValues(t, len(wants), len(tt.inputs))
	mc := testMetadataStore(testMetadata)
	st := startTs
	for i, page := range tt.inputs {
		b := newMetricBuilder(mc, true, nil, zap.NewNop())
		b.startTime = defaultBuilderStartTime // set to a non-zero value
		for _, pt := range page.pts {
			// set ts for testing
			pt.t = st
			assert.NoError(t, b.AddDataPoint(pt.lb, pt.t, pt.v))
		}
		md, err := b.getMetrics(pcommon.NewResource())
		assert.NoError(t, err)
		assertEquivalentMetrics(t, wants[i], md)
		st += interval
	}
}

type testDataPoint struct {
	lb labels.Labels
	t  int64
	v  float64
}

type testScrapedPage struct {
	pts []*testDataPoint
}

func createDataPoint(mname string, value float64, tagPairs ...string) *testDataPoint {
	var lbls []string
	lbls = append(lbls, tagPairs...)
	lbls = append(lbls, model.MetricNameLabel, mname)

	return &testDataPoint{
		lb: labels.FromStrings(lbls...),
		v:  value,
	}
}

func assertEquivalentMetrics(t *testing.T, want, got pmetric.Metrics) {
	require.Equal(t, want.ResourceMetrics().Len(), got.ResourceMetrics().Len())
	if want.ResourceMetrics().Len() == 0 {
		return
	}
	for i := 0; i < want.ResourceMetrics().Len(); i++ {
		wantSm := want.ResourceMetrics().At(i).ScopeMetrics()
		gotSm := got.ResourceMetrics().At(i).ScopeMetrics()
		require.Equal(t, wantSm.Len(), gotSm.Len())
		if wantSm.Len() == 0 {
			return
		}

		for j := 0; j < wantSm.Len(); j++ {
			wantMs := wantSm.At(j).Metrics()
			gotMs := gotSm.At(j).Metrics()
			require.Equal(t, wantMs.Len(), gotMs.Len())

			wmap := map[string]pmetric.Metric{}
			gmap := map[string]pmetric.Metric{}

			for k := 0; k < wantMs.Len(); k++ {
				wi := wantMs.At(k)
				wmap[wi.Name()] = wi
				gi := gotMs.At(k)
				gmap[gi.Name()] = gi
			}
			assert.EqualValues(t, wmap, gmap)
		}
	}

}
