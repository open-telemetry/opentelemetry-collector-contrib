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
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func runBuilderStartTimeTests(t *testing.T, tests []buildTestData,
	startTimeMetricRegex string, expectedBuilderStartTime float64) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := newMockMetadataCache(testMetadata)
			st := startTs
			for _, page := range tt.inputs {
				b := newMetricBuilder(mc, true, startTimeMetricRegex, zap.NewNop(), 0)
				b.startTime = defaultBuilderStartTime // set to a non-zero value
				for _, pt := range page.pts {
					// set ts for testing
					pt.t = st
					assert.NoError(t, b.AddDataPoint(pt.lb, pt.t, pt.v))
				}
				_, _, _, err := b.Build()
				assert.NoError(t, err)
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

	runBuilderStartTimeTests(t, matchTests, "^(.+_)*process_start_time_seconds$", matchBuilderStartTime)
	runBuilderStartTimeTests(t, nomatchTests, "^(.+_)*process_start_time_seconds$", defaultBuilderStartTime)
}

func TestGetBoundary(t *testing.T) {
	tests := []struct {
		name      string
		mtype     pmetric.MetricDataType
		labels    labels.Labels
		wantValue float64
		wantErr   string
	}{
		{
			name:  "cumulative histogram with bucket label",
			mtype: pmetric.MetricDataTypeHistogram,
			labels: labels.Labels{
				{Name: model.BucketLabel, Value: "0.256"},
			},
			wantValue: 0.256,
		},
		{
			name:  "gauge histogram with bucket label",
			mtype: pmetric.MetricDataTypeHistogram,
			labels: labels.Labels{
				{Name: model.BucketLabel, Value: "11.71"},
			},
			wantValue: 11.71,
		},
		{
			name:  "summary with bucket label",
			mtype: pmetric.MetricDataTypeSummary,
			labels: labels.Labels{
				{Name: model.BucketLabel, Value: "11.71"},
			},
			wantErr: "QuantileLabel is empty",
		},
		{
			name:  "summary with quantile label",
			mtype: pmetric.MetricDataTypeSummary,
			labels: labels.Labels{
				{Name: model.QuantileLabel, Value: "92.88"},
			},
			wantValue: 92.88,
		},
		{
			name:  "gauge histogram mismatched with bucket label",
			mtype: pmetric.MetricDataTypeSummary,
			labels: labels.Labels{
				{Name: model.BucketLabel, Value: "11.71"},
			},
			wantErr: "QuantileLabel is empty",
		},
		{
			name:  "other data types without matches",
			mtype: pmetric.MetricDataTypeGauge,
			labels: labels.Labels{
				{Name: model.BucketLabel, Value: "11.71"},
			},
			wantErr: "given metricType has no BucketLabel or QuantileLabel",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			value, err := getBoundary(tt.mtype, tt.labels)
			if tt.wantErr != "" {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}

			require.Nil(t, err)
			require.Equal(t, value, tt.wantValue)
		})
	}
}

func TestConvToMetricType(t *testing.T) {
	tests := []struct {
		name          string
		mtype         textparse.MetricType
		want          pmetric.MetricDataType
		wantMonotonic bool
	}{
		{
			name:          "textparse.counter",
			mtype:         textparse.MetricTypeCounter,
			want:          pmetric.MetricDataTypeSum,
			wantMonotonic: true,
		},
		{
			name:          "textparse.gauge",
			mtype:         textparse.MetricTypeGauge,
			want:          pmetric.MetricDataTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "textparse.unknown",
			mtype:         textparse.MetricTypeUnknown,
			want:          pmetric.MetricDataTypeGauge,
			wantMonotonic: false,
		},
		{
			name:          "textparse.histogram",
			mtype:         textparse.MetricTypeHistogram,
			want:          pmetric.MetricDataTypeHistogram,
			wantMonotonic: true,
		},
		{
			name:          "textparse.summary",
			mtype:         textparse.MetricTypeSummary,
			want:          pmetric.MetricDataTypeSummary,
			wantMonotonic: true,
		},
		{
			name:          "textparse.metric_type_info",
			mtype:         textparse.MetricTypeInfo,
			want:          pmetric.MetricDataTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "textparse.metric_state_set",
			mtype:         textparse.MetricTypeStateset,
			want:          pmetric.MetricDataTypeSum,
			wantMonotonic: false,
		},
		{
			name:          "textparse.metric_gauge_hostogram",
			mtype:         textparse.MetricTypeGaugeHistogram,
			want:          pmetric.MetricDataTypeNone,
			wantMonotonic: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, monotonic := convToMetricType(tt.mtype)
			require.Equal(t, got.String(), tt.want.String())
			require.Equal(t, monotonic, tt.wantMonotonic)
		})
	}
}

func TestIsUsefulLabel(t *testing.T) {
	tests := []struct {
		name      string
		mtypes    []pmetric.MetricDataType
		labelKeys []string
		want      bool
	}{
		{
			name: `unuseful "metric","instance","scheme","path","job" with any kind`,
			labelKeys: []string{
				model.MetricNameLabel, model.InstanceLabel, model.SchemeLabel, model.MetricsPathLabel, model.JobLabel,
			},
			mtypes: []pmetric.MetricDataType{
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeHistogram,
				pmetric.MetricDataTypeSummary,
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeNone,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeSum,
			},
			want: false,
		},
		{
			name: `bucket label with non "int_histogram", "histogram":: useful`,
			mtypes: []pmetric.MetricDataType{
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeSummary,
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeNone,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeSum,
			},
			labelKeys: []string{model.BucketLabel},
			want:      true,
		},
		{
			name: `quantile label with "summary": non-useful`,
			mtypes: []pmetric.MetricDataType{
				pmetric.MetricDataTypeSummary,
			},
			labelKeys: []string{model.QuantileLabel},
			want:      false,
		},
		{
			name:      `quantile label with non-"summary": useful`,
			labelKeys: []string{model.QuantileLabel},
			mtypes: []pmetric.MetricDataType{
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeHistogram,
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeNone,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeSum,
			},
			want: true,
		},
		{
			name:      `any other label with any type:: useful`,
			labelKeys: []string{"any_label", "foo.bar"},
			mtypes: []pmetric.MetricDataType{
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeHistogram,
				pmetric.MetricDataTypeSummary,
				pmetric.MetricDataTypeSum,
				pmetric.MetricDataTypeNone,
				pmetric.MetricDataTypeGauge,
				pmetric.MetricDataTypeSum,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			for _, mtype := range tt.mtypes {
				for _, labelKey := range tt.labelKeys {
					got := isUsefulLabel(mtype, labelKey)
					assert.Equal(t, got, tt.want)
				}
			}
		})
	}
}

type buildTestData struct {
	name   string
	inputs []*testScrapedPage
	wants  func() []*pmetric.MetricSlice
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				pt1 := sum.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(25.0)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().InsertString("foo", "other")

				return []*pmetric.MetricSlice{&mL}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				pt1 := sum0.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(25.0)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().InsertString("foo", "other")

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
				pt2.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL}
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
			mc := newMockMetadataCache(testMetadata)
			st := startTs
			for i, page := range tt.inputs {
				b := newMetricBuilder(mc, true, "", zap.NewNop(), startTs)
				b.startTime = defaultBuilderStartTime // set to a non-zero value
				b.intervalStartTimeMs = startTs
				for _, pt := range page.pts {
					// set ts for testing
					pt.t = st
					assert.NoError(t, b.AddDataPoint(pt.lb, pt.t, pt.v))
				}
				metrics, _, _, err := b.Build()
				assert.NoError(t, err)
				assertEquivalentMetrics(t, wants[i], metrics)
				st += interval
			}
		})
	}
}

func assertEquivalentMetrics(t *testing.T, want, got *pmetric.MetricSlice) {
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().InsertString("foo", "bar")

				mL1 := pmetric.NewMetricSlice()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(90.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(startTsPlusIntervalNanos)
				pt1.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0, &mL1}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().InsertString("foo", "bar")

				pt1 := gauge0.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(200.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().InsertString("bar", "foo")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("gauge_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().InsertString("foo", "bar")

				pt1 := gauge0.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(200.0)
				pt1.SetStartTimestamp(0)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().InsertString("bar", "foo")

				mL1 := pmetric.NewMetricSlice()
				m1 := mL1.AppendEmpty()
				m1.SetName("gauge_test")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt2 := gauge1.DataPoints().AppendEmpty()
				pt2.SetDoubleVal(20.0)
				pt2.SetStartTimestamp(0)
				pt2.SetTimestamp(startTsPlusIntervalNanos)
				pt2.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0, &mL1}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("unknown_test")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetStartTimestamp(0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("something_not_exists")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().InsertString("foo", "bar")

				m1 := mL0.AppendEmpty()
				m1.SetName("theother_not_exists")
				m1.SetDataType(pmetric.MetricDataTypeGauge)
				gauge1 := m1.Gauge()
				pt1 := gauge1.DataPoints().AppendEmpty()
				pt1.SetDoubleVal(200.0)
				pt1.SetTimestamp(startTsNanos)
				pt1.Attributes().InsertString("foo", "bar")

				pt2 := gauge1.DataPoints().AppendEmpty()
				pt2.SetDoubleVal(300.0)
				pt2.SetTimestamp(startTsNanos)
				pt2.Attributes().InsertString("bar", "foo")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("some_count")
				m0.SetDataType(pmetric.MetricDataTypeGauge)
				gauge0 := m0.Gauge()
				pt0 := gauge0.DataPoints().AppendEmpty()
				pt0.SetDoubleVal(100.0)
				pt0.SetTimestamp(startTsNanos)
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				pt1 := hist0.DataPoints().AppendEmpty()
				pt1.SetCount(3)
				pt1.SetSum(50)
				pt1.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt1.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1}))
				pt1.SetTimestamp(startTsNanos)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.Attributes().InsertString("key2", "v2")

				return []*pmetric.MetricSlice{&mL0}
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
						createDataPoint("hist_test2", 1, "le", "10"),
						createDataPoint("hist_test2", 2, "le", "20"),
						createDataPoint("hist_test2", 3, "le", "+inf"),
						createDataPoint("hist_test2_sum", 50),
						createDataPoint("hist_test2_count", 3),
					},
				},
			},
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				pt1 := hist0.DataPoints().AppendEmpty()
				pt1.SetCount(3)
				pt1.SetSum(50)
				pt1.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt1.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1}))
				pt1.SetTimestamp(startTsNanos)
				pt1.SetStartTimestamp(startTsNanos)
				pt1.Attributes().InsertString("key2", "v2")

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

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 3, "le", "+inf"),
						createDataPoint("hist_test_count", 3),
						createDataPoint("hist_test_sum", 100),
					},
				},
			},
			wants: func() []*pmetric.MetricSlice {
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

				return []*pmetric.MetricSlice{&mL0}
			},
		},
		{
			// this won't likely happen in real env, as prometheus wont generate histogram with less than 3 buckets
			name: "only-one-bucket-noninf",
			inputs: []*testScrapedPage{
				{
					pts: []*testDataPoint{
						createDataPoint("hist_test", 3, "le", "20"),
						createDataPoint("hist_test_count", 3),
						createDataPoint("hist_test_sum", 100),
					},
				},
			},
			wants: func() []*pmetric.MetricSlice {
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

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				m0 := mL0.AppendEmpty()
				m0.SetName("hist_test")
				m0.SetDataType(pmetric.MetricDataTypeHistogram)
				hist0 := m0.Histogram()
				hist0.SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				pt0 := hist0.DataPoints().AppendEmpty()
				pt0.SetCount(3)
				pt0.SetSum(0)
				pt0.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{10, 20}))
				pt0.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1}))
				pt0.SetTimestamp(startTsNanos)
				pt0.SetStartTimestamp(startTsNanos)
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
				mL0 := pmetric.NewMetricSlice()
				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")
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

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")

				return []*pmetric.MetricSlice{&mL0}
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
			wants: func() []*pmetric.MetricSlice {
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
				pt0.Attributes().InsertString("foo", "bar")
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

				return []*pmetric.MetricSlice{&mL0}
			},
		},
	}

	runBuilderTests(t, tests)
}

// Ensure that we reject duplicate label keys. See https://github.com/open-telemetry/wg-prometheus/issues/44.
func TestOTLPMetricBuilderDuplicateLabelKeysAreRejected(t *testing.T) {
	mc := newMockMetadataCache(testMetadata)
	mb := newMetricBuilder(mc, true, "", zap.NewNop(), 0)

	dupLabels := labels.Labels{
		{Name: "__name__", Value: "test"},
		{Name: "a", Value: "1"},
		{Name: "a", Value: "1"},
		{Name: "z", Value: "9"},
		{Name: "z", Value: "1"},
		{Name: "instance", Value: "0.0.0.0:8855"},
		{Name: "job", Value: "test"},
	}

	err := mb.AddDataPoint(dupLabels, 1917, 1.0)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), `invalid sample: non-unique label names: ["a" "z"]`)
}

func Test_OTLPMetricBuilder_baddata(t *testing.T) {
	t.Run("empty-metric-name", func(t *testing.T) {
		b := newMetricBuilder(newMockMetadataCache(testMetadata), true, "", zap.NewNop(), 0)
		b.startTime = 1.0 // set to a non-zero value
		if err := b.AddDataPoint(labels.FromStrings("a", "b"), startTs, 123); !errors.Is(err, errMetricNameNotFound) {
			t.Error("expecting errMetricNameNotFound error, but get nil")
			return
		}

		if _, _, _, err := b.Build(); !errors.Is(err, errNoDataToBuild) {
			t.Error("expecting errNoDataToBuild error, but get nil")
		}
	})

	t.Run("histogram-datapoint-no-bucket-label", func(t *testing.T) {
		b := newMetricBuilder(newMockMetadataCache(testMetadata), true, "", zap.NewNop(), 0)
		b.startTime = 1.0 // set to a non-zero value
		if err := b.AddDataPoint(createLabels("hist_test", "k", "v"), startTs, 123); !errors.Is(err, errEmptyBoundaryLabel) {
			t.Error("expecting errEmptyBoundaryLabel error, but get nil")
		}
	})

	t.Run("summary-datapoint-no-quantile-label", func(t *testing.T) {
		b := newMetricBuilder(newMockMetadataCache(testMetadata), true, "", zap.NewNop(), 0)
		b.startTime = 1.0 // set to a non-zero value
		if err := b.AddDataPoint(createLabels("summary_test", "k", "v"), startTs, 123); !errors.Is(err, errEmptyBoundaryLabel) {
			t.Error("expecting errEmptyBoundaryLabel error, but get nil")
		}
	})
}

func newMockMetadataCache(data map[string]scrape.MetricMetadata) *mockMetadataCache {
	return &mockMetadataCache{data: data}
}

type mockMetadataCache struct {
	data map[string]scrape.MetricMetadata
}

func (m *mockMetadataCache) Metadata(metricName string) (scrape.MetricMetadata, bool) {
	mm, ok := m.data[metricName]
	return mm, ok
}

func (m *mockMetadataCache) SharedLabels() labels.Labels {
	return labels.FromStrings("__scheme__", "http")
}
