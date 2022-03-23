// Copyright OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package signalfx

import (
	"math"
	"sort"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
)

const (
	unixSecs  = int64(1574092046)
	unixNSecs = int64(11 * time.Millisecond)
	tsMSecs   = unixSecs*1e3 + unixNSecs/1e6
)

func Test_FromMetrics(t *testing.T) {
	labelMap := map[string]interface{}{
		"k0": "v0",
		"k1": "v1",
	}

	ts := pdata.NewTimestampFromTime(time.Unix(unixSecs, unixNSecs))

	const doubleVal = 1234.5678
	initDoublePt := func(doublePt pdata.NumberDataPoint) {
		doublePt.SetTimestamp(ts)
		doublePt.SetDoubleVal(doubleVal)
	}

	initDoublePtWithLabels := func(doublePtWithLabels pdata.NumberDataPoint) {
		initDoublePt(doublePtWithLabels)
		pdata.NewMapFromRaw(labelMap).CopyTo(doublePtWithLabels.Attributes())
	}

	const int64Val = int64(123)
	initInt64Pt := func(int64Pt pdata.NumberDataPoint) {
		int64Pt.SetTimestamp(ts)
		int64Pt.SetIntVal(int64Val)
	}

	initInt64PtWithLabels := func(int64PtWithLabels pdata.NumberDataPoint) {
		initInt64Pt(int64PtWithLabels)
		pdata.NewMapFromRaw(labelMap).CopyTo(int64PtWithLabels.Attributes())
	}

	histBounds := []float64{1, 2, 4}
	histCounts := []uint64{4, 2, 3, 7}

	initHistDP := func(histDP pdata.HistogramDataPoint) {
		histDP.SetTimestamp(ts)
		histDP.SetCount(16)
		histDP.SetSum(100.0)
		histDP.SetExplicitBounds(histBounds)
		histDP.SetBucketCounts(histCounts)
		pdata.NewMapFromRaw(labelMap).CopyTo(histDP.Attributes())
	}
	histDP := pdata.NewHistogramDataPoint()
	initHistDP(histDP)

	initHistDPNoBuckets := func(histDP pdata.HistogramDataPoint) {
		histDP.SetCount(2)
		histDP.SetSum(10)
		histDP.SetTimestamp(ts)
		pdata.NewMapFromRaw(labelMap).CopyTo(histDP.Attributes())
	}
	histDPNoBuckets := pdata.NewHistogramDataPoint()
	initHistDPNoBuckets(histDPNoBuckets)

	const summarySumVal = 123.4
	const summaryCountVal = 111

	initSummaryDP := func(summaryDP pdata.SummaryDataPoint) {
		summaryDP.SetTimestamp(ts)
		summaryDP.SetSum(summarySumVal)
		summaryDP.SetCount(summaryCountVal)
		qvs := summaryDP.QuantileValues()
		for i := 0; i < 4; i++ {
			qv := qvs.AppendEmpty()
			qv.SetQuantile(0.25 * float64(i+1))
			qv.SetValue(float64(i))
		}
		pdata.NewMapFromRaw(labelMap).CopyTo(summaryDP.Attributes())
	}

	initEmptySummaryDP := func(summaryDP pdata.SummaryDataPoint) {
		summaryDP.SetTimestamp(ts)
		summaryDP.SetSum(summarySumVal)
		summaryDP.SetCount(summaryCountVal)
		pdata.NewMapFromRaw(labelMap).CopyTo(summaryDP.Attributes())
	}

	tests := []struct {
		name              string
		metricsFn         func() pdata.Metrics
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "nil_node_nil_resources_no_dims",
			metricsFn: func() pdata.Metrics {
				out := pdata.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeGauge)
					initDoublePt(m.Gauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeGauge)
					initInt64Pt(m.Gauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
					initInt64Pt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("delta_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("delta_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
					initInt64Pt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_sum_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(false)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_sum_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(false)
					initInt64Pt(m.Sum().DataPoints().AppendEmpty())
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, nil, doubleVal),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, nil, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, nil, doubleVal),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, nil, int64Val),
				doubleSFxDataPoint("delta_double_with_dims", &sfxMetricTypeCounter, nil, doubleVal),
				int64SFxDataPoint("delta_int_with_dims", &sfxMetricTypeCounter, nil, int64Val),
				doubleSFxDataPoint("gauge_sum_double_with_dims", &sfxMetricTypeGauge, nil, doubleVal),
				int64SFxDataPoint("gauge_sum_int_with_dims", &sfxMetricTypeGauge, nil, int64Val),
			},
		},
		{
			name: "nil_node_and_resources_with_dims",
			metricsFn: func() pdata.Metrics {
				out := pdata.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeGauge)
					initDoublePtWithLabels(m.Gauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeGauge)
					initInt64PtWithLabels(m.Gauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					initDoublePtWithLabels(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					initInt64PtWithLabels(m.Sum().DataPoints().AppendEmpty())
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, labelMap, doubleVal),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, labelMap, doubleVal),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, labelMap, int64Val),
			},
		},
		{
			name: "with_node_resources_dims",
			metricsFn: func() pdata.Metrics {
				out := pdata.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().InsertString("k_r0", "v_r0")
				res.Attributes().InsertString("k_r1", "v_r1")
				res.Attributes().InsertString("k_n0", "v_n0")
				res.Attributes().InsertString("k_n1", "v_n1")

				ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
				ilm.Metrics().EnsureCapacity(2)

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeGauge)
					initDoublePtWithLabels(m.Gauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeGauge)
					initInt64PtWithLabels(m.Gauge().DataPoints().AppendEmpty())
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(map[string]interface{}{
						"k_n0": "v_n0",
						"k_n1": "v_n1",
						"k_r0": "v_r0",
						"k_r1": "v_r1",
					}, labelMap),
					doubleVal),
				int64SFxDataPoint(
					"gauge_int_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(map[string]interface{}{
						"k_n0": "v_n0",
						"k_n1": "v_n1",
						"k_r0": "v_r0",
						"k_r1": "v_r1",
					}, labelMap),
					int64Val),
			},
		},
		{
			name: "histograms",
			metricsFn: func() pdata.Metrics {
				out := pdata.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("double_histo")
					m.SetDataType(pdata.MetricDataTypeHistogram)
					initHistDP(m.Histogram().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("double_delta_histo")
					m.SetDataType(pdata.MetricDataTypeHistogram)
					m.Histogram().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
					initHistDP(m.Histogram().DataPoints().AppendEmpty())
				}
				return out
			},
			wantSfxDataPoints: mergeDPs(
				expectedFromHistogram("double_histo", labelMap, histDP, false),
				expectedFromHistogram("double_delta_histo", labelMap, histDP, true),
			),
		},
		{
			name: "distribution_no_buckets",
			metricsFn: func() pdata.Metrics {
				out := pdata.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("no_bucket_histo")
				m.SetDataType(pdata.MetricDataTypeHistogram)
				initHistDPNoBuckets(m.Histogram().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: expectedFromHistogram("no_bucket_histo", labelMap, histDPNoBuckets, false),
		},
		{
			name: "summaries",
			metricsFn: func() pdata.Metrics {
				out := pdata.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("summary")
				m.SetDataType(pdata.MetricDataTypeSummary)
				initSummaryDP(m.Summary().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: expectedFromSummary("summary", labelMap, summaryCountVal, summarySumVal),
		},
		{
			name: "empty_summary",
			metricsFn: func() pdata.Metrics {
				out := pdata.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("empty_summary")
				m.SetDataType(pdata.MetricDataTypeSummary)
				initEmptySummaryDP(m.Summary().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: expectedFromEmptySummary("empty_summary", labelMap, summaryCountVal, summarySumVal),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := tt.metricsFn()
			gotSfxDataPoints, err := FromMetrics(md)
			require.NoError(t, err)
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.Equal(t, tt.wantSfxDataPoints, gotSfxDataPoints)
		})
	}
}

func sortDimensions(points []*sfxpb.DataPoint) {
	for _, point := range points {
		if point.Dimensions == nil {
			continue
		}
		sort.Slice(point.Dimensions, func(i, j int) bool {
			return point.Dimensions[i].Key < point.Dimensions[j].Key
		})
	}
}

func doubleSFxDataPoint(
	metric string,
	metricType *sfxpb.MetricType,
	dims map[string]interface{},
	val float64,
) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     metric,
		Timestamp:  tsMSecs,
		Value:      sfxpb.Datum{DoubleValue: &val},
		MetricType: metricType,
		Dimensions: sfxDimensions(dims),
	}
}

func int64SFxDataPoint(
	metric string,
	metricType *sfxpb.MetricType,
	dims map[string]interface{},
	val int64,
) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     metric,
		Timestamp:  tsMSecs,
		Value:      sfxpb.Datum{IntValue: &val},
		MetricType: metricType,
		Dimensions: sfxDimensions(dims),
	}
}

func sfxDimensions(m map[string]interface{}) []*sfxpb.Dimension {
	sfxDims := make([]*sfxpb.Dimension, 0, len(m))
	for k, v := range m {
		sfxDims = append(sfxDims, &sfxpb.Dimension{
			Key:   k,
			Value: v.(string),
		})
	}

	return sfxDims
}

func expectedFromHistogram(
	metricName string,
	dims map[string]interface{},
	histDP pdata.HistogramDataPoint,
	isDelta bool,
) []*sfxpb.DataPoint {
	buckets := histDP.BucketCounts()

	dps := make([]*sfxpb.DataPoint, 0)

	typ := &sfxMetricTypeCumulativeCounter
	if isDelta {
		typ = &sfxMetricTypeCounter
	}

	dps = append(dps,
		int64SFxDataPoint(metricName+"_count", typ, dims, int64(histDP.Count())),
		doubleSFxDataPoint(metricName, typ, dims, histDP.Sum()))

	explicitBounds := histDP.ExplicitBounds()
	if explicitBounds == nil {
		return dps
	}
	for i := 0; i < len(explicitBounds); i++ {
		dimsCopy := maps.CloneRawMap(dims)
		dimsCopy[upperBoundDimensionKey] = float64ToDimValue(explicitBounds[i])
		dps = append(dps, int64SFxDataPoint(metricName+"_bucket", typ, dimsCopy, int64(buckets[i])))
	}
	dimsCopy := maps.CloneRawMap(dims)
	dimsCopy[upperBoundDimensionKey] = float64ToDimValue(math.Inf(1))
	dps = append(dps, int64SFxDataPoint(metricName+"_bucket", typ, dimsCopy, int64(buckets[len(buckets)-1])))
	return dps
}

func expectedFromSummary(name string, labelMap map[string]interface{}, count int64, sumVal float64) []*sfxpb.DataPoint {
	countName := name + "_count"
	countPt := int64SFxDataPoint(countName, &sfxMetricTypeCumulativeCounter, labelMap, count)
	sumPt := doubleSFxDataPoint(name, &sfxMetricTypeCumulativeCounter, labelMap, sumVal)
	out := []*sfxpb.DataPoint{countPt, sumPt}
	quantileDimVals := []string{"0.25", "0.5", "0.75", "1"}
	for i := 0; i < 4; i++ {
		qDims := map[string]interface{}{"quantile": quantileDimVals[i]}
		qPt := doubleSFxDataPoint(
			name+"_quantile",
			&sfxMetricTypeGauge,
			maps.MergeRawMaps(labelMap, qDims),
			float64(i),
		)
		out = append(out, qPt)
	}
	return out
}

func expectedFromEmptySummary(name string, labelMap map[string]interface{}, count int64,
	sumVal float64) []*sfxpb.DataPoint {
	countName := name + "_count"
	countPt := int64SFxDataPoint(countName, &sfxMetricTypeCumulativeCounter, labelMap, count)
	sumPt := doubleSFxDataPoint(name, &sfxMetricTypeCumulativeCounter, labelMap, sumVal)
	return []*sfxpb.DataPoint{countPt, sumPt}
}

func mergeDPs(dps ...[]*sfxpb.DataPoint) []*sfxpb.DataPoint {
	var out []*sfxpb.DataPoint
	for i := range dps {
		out = append(out, dps[i]...)
	}
	return out
}
