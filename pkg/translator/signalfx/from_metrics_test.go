// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfx

import (
	"sort"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
)

const (
	unixSecs  = int64(1574092046)
	unixNSecs = int64(11 * time.Millisecond)
	tsMSecs   = unixSecs*1e3 + unixNSecs/1e6
)

func Test_FromMetrics(t *testing.T) {
	labelMap := map[string]string{
		"k0": "v0",
		"k1": "v1",
	}
	attrMap := pcommon.NewMap()
	attrMap.PutStr("k0", "v0")
	attrMap.PutStr("k1", "v1")

	ts := pcommon.NewTimestampFromTime(time.Unix(unixSecs, unixNSecs))

	const doubleVal = 1234.5678
	initDoublePt := func(dp pmetric.NumberDataPoint) {
		dp.SetTimestamp(ts)
		dp.SetDoubleValue(doubleVal)
	}

	initDoublePtWithLabels := func(dp pmetric.NumberDataPoint) {
		initDoublePt(dp)
		attrMap.CopyTo(dp.Attributes())
	}

	const int64Val = int64(123)
	initInt64Pt := func(dp pmetric.NumberDataPoint) {
		dp.SetTimestamp(ts)
		dp.SetIntValue(int64Val)
	}

	initInt64PtWithLabels := func(dp pmetric.NumberDataPoint) {
		initInt64Pt(dp)
		attrMap.CopyTo(dp.Attributes())
	}

	initHistDPNoOptional := func(dp pmetric.HistogramDataPoint) {
		dp.SetTimestamp(ts)
		dp.SetCount(16)
		dp.ExplicitBounds().FromRaw([]float64{1, 2, 4})
		dp.BucketCounts().FromRaw([]uint64{4, 2, 3, 7})
		attrMap.CopyTo(dp.Attributes())
	}

	initHistDP := func(dp pmetric.HistogramDataPoint) {
		initHistDPNoOptional(dp)
		dp.SetSum(100.0)
		dp.SetMin(0.1)
		dp.SetMax(11.11)
	}

	tests := []struct {
		name              string
		metricsFn         func() pmetric.Metrics
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "no_resource_no_attributes",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					initDoublePt(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_int_with_dims")
					initInt64Pt(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_double_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
					initInt64Pt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("delta_double_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("delta_int_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					initInt64Pt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_sum_double_with_dims")
					m.SetEmptySum().SetIsMonotonic(false)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_sum_int_with_dims")
					m.SetEmptySum().SetIsMonotonic(false)
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
			name: "no_resources_with_attributes",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_int_with_dims")
					initInt64PtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_double_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
					initDoublePtWithLabels(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
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
			name: "with_resources_with_attributes",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k_r0", "v_r0")
				res.Attributes().PutStr("k_r1", "v_r1")
				res.Attributes().PutStr("k_n0", "v_n0")
				res.Attributes().PutStr("k_n1", "v_n1")

				ilm := rm.ScopeMetrics().AppendEmpty()
				ilm.Metrics().EnsureCapacity(2)

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_int_with_dims")
					initInt64PtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeStringMaps(map[string]string{
						"k_n0": "v_n0",
						"k_n1": "v_n1",
						"k_r0": "v_r0",
						"k_r1": "v_r1",
					}, labelMap),
					doubleVal),
				int64SFxDataPoint(
					"gauge_int_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeStringMaps(map[string]string{
						"k_n0": "v_n0",
						"k_n1": "v_n1",
						"k_r0": "v_r0",
						"k_r1": "v_r1",
					}, labelMap),
					int64Val),
			},
		},
		{
			name: "histogram",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("histogram")
				m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				initHistDP(m.Histogram().DataPoints().AppendEmpty())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("histogram_count", &sfxMetricTypeCumulativeCounter, labelMap, 16),
				doubleSFxDataPoint("histogram_sum", &sfxMetricTypeCumulativeCounter, labelMap, 100.0),
				doubleSFxDataPoint("histogram_min", &sfxMetricTypeGauge, labelMap, 0.1),
				doubleSFxDataPoint("histogram_max", &sfxMetricTypeGauge, labelMap, 11.11),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "1"}, labelMap), 4),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "2"}, labelMap), 6),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "4"}, labelMap), 9),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "+Inf"}, labelMap), 16),
			},
		},
		{
			name: "histogram_no_optional",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("histogram")
				m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				initHistDPNoOptional(m.Histogram().DataPoints().AppendEmpty())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("histogram_count", &sfxMetricTypeCumulativeCounter, labelMap, 16),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "1"}, labelMap), 4),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "2"}, labelMap), 6),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "4"}, labelMap), 9),
				int64SFxDataPoint("histogram_bucket", &sfxMetricTypeCumulativeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "+Inf"}, labelMap), 16),
			},
		},
		{
			name: "delta_histogram",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("delta_histogram")
				m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				initHistDP(m.Histogram().DataPoints().AppendEmpty())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("delta_histogram_count", &sfxMetricTypeCounter, labelMap, 16),
				doubleSFxDataPoint("delta_histogram_sum", &sfxMetricTypeCounter, labelMap, 100.0),
				doubleSFxDataPoint("delta_histogram_min", &sfxMetricTypeGauge, labelMap, 0.1),
				doubleSFxDataPoint("delta_histogram_max", &sfxMetricTypeGauge, labelMap, 11.11),
				int64SFxDataPoint("delta_histogram_bucket", &sfxMetricTypeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "1"}, labelMap), 4),
				int64SFxDataPoint("delta_histogram_bucket", &sfxMetricTypeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "2"}, labelMap), 6),
				int64SFxDataPoint("delta_histogram_bucket", &sfxMetricTypeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "4"}, labelMap), 9),
				int64SFxDataPoint("delta_histogram_bucket", &sfxMetricTypeCounter,
					maps.MergeStringMaps(map[string]string{bucketDimensionKey: "+Inf"}, labelMap), 16),
			},
		},
		{
			name: "distribution_no_buckets",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("no_bucket_histo")
				dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
				dp.SetCount(2)
				dp.SetSum(10)
				dp.SetTimestamp(ts)
				attrMap.CopyTo(dp.Attributes())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("no_bucket_histo_count", &sfxMetricTypeCumulativeCounter, labelMap, 2),
				doubleSFxDataPoint("no_bucket_histo_sum", &sfxMetricTypeCumulativeCounter, labelMap, 10),
			},
		},
		{
			name: "summaries",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("summary")
				dp := m.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)
				dp.SetSum(123.4)
				dp.SetCount(111)
				qvs := dp.QuantileValues()
				for i := 0; i < 4; i++ {
					qv := qvs.AppendEmpty()
					qv.SetQuantile(0.25 * float64(i+1))
					qv.SetValue(float64(i))
				}
				attrMap.CopyTo(dp.Attributes())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("summary_count", &sfxMetricTypeCumulativeCounter, labelMap, 111),
				doubleSFxDataPoint("summary_sum", &sfxMetricTypeCumulativeCounter, labelMap, 123.4),
				doubleSFxDataPoint("summary_quantile", &sfxMetricTypeGauge,
					maps.MergeStringMaps(map[string]string{quantileDimensionKey: "0.25"}, labelMap), 0),
				doubleSFxDataPoint("summary_quantile", &sfxMetricTypeGauge,
					maps.MergeStringMaps(map[string]string{quantileDimensionKey: "0.5"}, labelMap), 1),
				doubleSFxDataPoint("summary_quantile", &sfxMetricTypeGauge,
					maps.MergeStringMaps(map[string]string{quantileDimensionKey: "0.75"}, labelMap), 2),
				doubleSFxDataPoint("summary_quantile", &sfxMetricTypeGauge,
					maps.MergeStringMaps(map[string]string{quantileDimensionKey: "1"}, labelMap), 3),
			},
		},
		{
			name: "empty_summary",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("empty_summary")
				dp := m.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)
				dp.SetSum(12.3)
				dp.SetCount(11)
				attrMap.CopyTo(dp.Attributes())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("empty_summary_count", &sfxMetricTypeCumulativeCounter, labelMap, 11),
				doubleSFxDataPoint("empty_summary_sum", &sfxMetricTypeCumulativeCounter, labelMap, 12.3),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from := &FromTranslator{}
			gotSfxDataPoints, err := from.FromMetrics(tt.metricsFn(), false)
			require.NoError(t, err)
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.EqualValues(t, tt.wantSfxDataPoints, gotSfxDataPoints)
		})
	}

	testsWithDropHistogramBuckets := []struct {
		name              string
		metricsFn         func() pmetric.Metrics
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "histogram",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("histogram")
				m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				initHistDP(m.Histogram().DataPoints().AppendEmpty())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("histogram_count", &sfxMetricTypeCumulativeCounter, labelMap, 16),
				doubleSFxDataPoint("histogram_sum", &sfxMetricTypeCumulativeCounter, labelMap, 100.0),
				doubleSFxDataPoint("histogram_min", &sfxMetricTypeGauge, labelMap, 0.1),
				doubleSFxDataPoint("histogram_max", &sfxMetricTypeGauge, labelMap, 11.11),
			},
		},
		{
			name: "histogram_no_optional",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("histogram")
				m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				initHistDPNoOptional(m.Histogram().DataPoints().AppendEmpty())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("histogram_count", &sfxMetricTypeCumulativeCounter, labelMap, 16),
			},
		},
		{
			name: "delta_histogram",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("delta_histogram")
				m.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				initHistDP(m.Histogram().DataPoints().AppendEmpty())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("delta_histogram_count", &sfxMetricTypeCounter, labelMap, 16),
				doubleSFxDataPoint("delta_histogram_sum", &sfxMetricTypeCounter, labelMap, 100.0),
				doubleSFxDataPoint("delta_histogram_min", &sfxMetricTypeGauge, labelMap, 0.1),
				doubleSFxDataPoint("delta_histogram_max", &sfxMetricTypeGauge, labelMap, 11.11),
			},
		},
		{
			name: "distribution_no_buckets",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("no_bucket_histo")
				dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
				dp.SetCount(2)
				dp.SetSum(10)
				dp.SetTimestamp(ts)
				attrMap.CopyTo(dp.Attributes())
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("no_bucket_histo_count", &sfxMetricTypeCumulativeCounter, labelMap, 2),
				doubleSFxDataPoint("no_bucket_histo_sum", &sfxMetricTypeCumulativeCounter, labelMap, 10),
			},
		},
	}
	for _, tt := range testsWithDropHistogramBuckets {
		t.Run(tt.name, func(t *testing.T) {
			from := &FromTranslator{}
			gotSfxDataPoints, err := from.FromMetrics(tt.metricsFn(), true)
			require.NoError(t, err)
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.EqualValues(t, tt.wantSfxDataPoints, gotSfxDataPoints)
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
	dims map[string]string,
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
	dims map[string]string,
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

func sfxDimensions(m map[string]string) []*sfxpb.Dimension {
	sfxDims := make([]*sfxpb.Dimension, 0, len(m))
	for k, v := range m {
		sfxDims = append(sfxDims, &sfxpb.Dimension{Key: k, Value: v})
	}

	return sfxDims
}
