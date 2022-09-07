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
	"errors"
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

func runBuilderStartTimeTests(t *testing.T, tests []buildTestData, startTimeMetricRegex *regexp.Regexp, expectedBuilderStartTime float64) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := testMetadataStore(testMetadata)
			st := startTs
			for _, page := range tt.inputs {
				b := newMetricBuilder(mc, true, startTimeMetricRegex, zap.NewNop())
				b.startTime = defaultBuilderStartTime // set to a non-zero value
				for _, pt := range page.pts {
					// set ts for testing
					pt.t = st
					assert.NoError(t, b.AddDataPoint(pt.lb, pt.t, pt.v))
				}
				assert.NoError(t, b.appendMetrics(pmetric.NewMetricSlice()))
				assert.EqualValues(t, b.startTime, expectedBuilderStartTime)
				st += interval
			}
		})
	}
}

func Test_startTimeMetricMatch_pdata(t *testing.T) {
	matchBuilderStartTime := 123.456
	matchTests := []buildTestData{
		{
			name: "prefix_match",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("example_process_start_time_seconds",
							matchBuilderStartTime, "foo", "bar"),
					},
				},
			},
		},
		{
			name: "match",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("process_start_time_seconds",
							matchBuilderStartTime, "foo", "bar"),
					},
				},
			},
		},
	}
	nomatchTests := []buildTestData{
		{
			name: "nomatch1",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("_process_start_time_seconds",
							matchBuilderStartTime, "foo", "bar"),
					},
				},
			},
		},
		{
			name: "nomatch2",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("subprocess_start_time_seconds",
							matchBuilderStartTime, "foo", "bar"),
					},
				},
			},
		},
	}

	runBuilderStartTimeTests(t, matchTests, regexp.MustCompile("^(.+_)*process_start_time_seconds$"), matchBuilderStartTime)
	runBuilderStartTimeTests(t, nomatchTests, regexp.MustCompile("^(.+_)*process_start_time_seconds$"), defaultBuilderStartTime)
}

type buildTestData struct {
	name   string
	inputs []*testScrapedPage
	wants  func() []pmetric.MetricSlice
}

func Test_OTLPMetricBuilder_counters(t *testing.T) {
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
			wants: func() []pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
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

				return []pmetric.MetricSlice{mL}
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
			wants: func() []pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
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

				return []pmetric.MetricSlice{mL}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL := pmetric.NewMetricSlice()
				m0 := mL.AppendEmpty()
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

				return []pmetric.MetricSlice{mL}
			},
		},
	}

	runBuilderTests(t, tests)
}

func runBuilderTests(t *testing.T, tests []buildTestData) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
				metrics := pmetric.NewMetricSlice()
				assert.NoError(t, b.appendMetrics(metrics))
				assertEquivalentMetrics(t, wants[i], metrics)
				st += interval
			}
		})
	}
}

func assertEquivalentMetrics(t *testing.T, want, got pmetric.MetricSlice) {
	if !assert.Equal(t, want.Len(), got.Len()) {
		return
	}
	wmap := map[string]pmetric.Metric{}
	gmap := map[string]pmetric.Metric{}

	for i := 0; i < want.Len(); i++ {
		wi := want.At(i)
		wmap[wi.Name()] = wi
		gi := got.At(i)
		gmap[gi.Name()] = gi
	}

	assert.EqualValues(t, wmap, gmap)
}

var (
	startTsNanos             = pcommon.Timestamp(startTs * 1e6)
	startTsPlusIntervalNanos = pcommon.Timestamp((startTs + interval) * 1e6)
)

func Test_OTLPMetricBuilder_gauges(t *testing.T) {
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				mL1 := pmetric.NewMetricSlice()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(90.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(startTsPlusIntervalNanos)
				pt1.Attributes().UpsertString("foo", "bar")

				return []pmetric.MetricSlice{mL0, mL1}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				mL1 := pmetric.NewMetricSlice()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt2 := gauge1.DataPoints().AppendEmpty()
				pt2.SetDoubleVal(20.0)
				pt2.SetStartTimestamp(0)
				pt2.SetTimestamp(startTsPlusIntervalNanos)
				pt2.Attributes().UpsertString("foo", "bar")

				return []pmetric.MetricSlice{mL0, mL1}
			},
		},
	}

	runBuilderTests(t, tests)
}

func Test_OTLPMetricBuilder_untype(t *testing.T) {
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("unknown_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("some_count")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().UpsertString("foo", "bar")

				return []pmetric.MetricSlice{mL0}
			},
		},
	}

	runBuilderTests(t, tests)
}

func Test_OTLPMetricBuilder_histogram(t *testing.T) {
	tests := []buildTestData{
		{
			name: "single item",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
			},
		},
		{
			name: "multi-groups",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
						createDataPoint("hist_test", 1, "key2", "v2", "le", "10"),
						createDataPoint("hist_test", 2, "key2", "v2", "le", "20"),
						createDataPoint("hist_test", 3, "key2", "v2", "le", "+inf"),
						createDataPoint("hist_test_sum", 50, "key2", "v2"),
						createDataPoint("hist_test_count", 3, "key2", "v2"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
			},
		},
		{
			name: "multi-groups-and-families",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
						createDataPoint("hist_test", 1, "key2", "v2", "le", "10"),
						createDataPoint("hist_test", 2, "key2", "v2", "le", "20"),
						createDataPoint("hist_test", 3, "key2", "v2", "le", "+inf"),
						createDataPoint("hist_test_sum", 50, "key2", "v2"),
						createDataPoint("hist_test_count", 3, "key2", "v2"),
						createDataPoint("hist_test2", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test2", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test2", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test2_sum", 50, "foo", "bar"),
						createDataPoint("hist_test2_count", 3, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
			},
		},
		{
			name: "unordered-buckets",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 10, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
						createDataPoint("hist_test_count", 10, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_count", 3, "foo", "bar"),
						createDataPoint("hist_test_sum", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket-noninf",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 3, "foo", "bar", "le", "20"),
						createDataPoint("hist_test_count", 3, "foo", "bar"),
						createDataPoint("hist_test_sum", 100, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
			},
		},
		{
			name: "no-sum",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_count", 3, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []pmetric.MetricSlice{mL0}
			},
		},
		{
			name: "corrupted-no-count",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 1, "foo", "bar", "le", "10"),
						createDataPoint("hist_test", 2, "foo", "bar", "le", "20"),
						createDataPoint("hist_test", 3, "foo", "bar", "le", "+inf"),
						createDataPoint("hist_test_sum", 99, "foo", "bar"),
					},
				},
			},
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []pmetric.MetricSlice{mL0}
			},
		},
	}

	runBuilderTests(t, tests)
}

func Test_OTLPMetricBuilder_summary(t *testing.T) {
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
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
			wants: func() []pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
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

				return []pmetric.MetricSlice{mL0}
			},
		},
	}

	runBuilderTests(t, tests)
}

// Ensure that we reject duplicate label keys. See https://github.com/open-telemetry/wg-prometheus/issues/44.
func TestOTLPMetricBuilderDuplicateLabelKeysAreRejected(t *testing.T) {
	mc := testMetadataStore(testMetadata)
	mb := newMetricBuilder(mc, true, nil, zap.NewNop())

	dupLabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "foo",
		"a", "1",
		"a", "1",
		"z", "9",
		"z", "1",
	)

	err := mb.AddDataPoint(dupLabels, 1917, 1.0)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `invalid sample: non-unique label names: ["a" "z"]`)
}

func Test_OTLPMetricBuilder_baddata(t *testing.T) {
	t.Run("empty-metric-name", func(t *testing.T) {
		b := newMetricBuilder(testMetadataStore(testMetadata), true, nil, zap.NewNop())
		b.startTime = 1.0 // set to a non-zero value
		if err := b.AddDataPoint(labels.FromStrings("a", "b"), startTs, 123); !errors.Is(err, errMetricNameNotFound) {
			t.Error("expecting errMetricNameNotFound error, but get nil")
			return
		}

		if err := b.appendMetrics(pmetric.NewMetricSlice()); !errors.Is(err, errNoDataToBuild) {
			t.Error("expecting errNoDataToBuild error, but get nil")
		}
	})

	t.Run("histogram-datapoint-no-bucket-label", func(t *testing.T) {
		b := newMetricBuilder(testMetadataStore(testMetadata), true, nil, zap.NewNop())
		b.startTime = 1.0 // set to a non-zero value
		if err := b.AddDataPoint(labels.FromStrings(model.MetricNameLabel, "hist_test", "k", "v"), startTs, 123); !errors.Is(err, errEmptyLeLabel) {
			t.Error("expecting errEmptyBoundaryLabel error, but get nil")
		}
	})

	t.Run("summary-datapoint-no-quantile-label", func(t *testing.T) {
		b := newMetricBuilder(testMetadataStore(testMetadata), true, nil, zap.NewNop())
		b.startTime = 1.0 // set to a non-zero value
		if err := b.AddDataPoint(labels.FromStrings(model.MetricNameLabel, "summary_test", "k", "v"), startTs, 123); !errors.Is(err, errEmptyQuantileLabel) {
			t.Error("expecting errEmptyBoundaryLabel error, but get nil")
		}
	})
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
