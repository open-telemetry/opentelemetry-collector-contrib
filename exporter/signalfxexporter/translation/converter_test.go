// Copyright 2019 OpenTelemetry Authors
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

package translation

import (
	"math"
	"sort"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/util"
)

func Test_MetricDataToSignalFxV2(t *testing.T) {
	logger := zap.NewNop()

	labelMap := map[string]string{
		"k0": "v0",
		"k1": "v1",
	}
	labels := pdata.NewStringMap()
	labels.InitFromMap(labelMap)

	unixSecs := int64(1574092046)
	unixNSecs := int64(11 * time.Millisecond)
	ts := pdata.TimestampUnixNano(time.Unix(unixSecs, unixNSecs).UnixNano())
	tsMSecs := unixSecs*1e3 + unixNSecs/1e6

	doubleVal := 1234.5678
	doublePt := pdata.NewDoubleDataPoint()
	doublePt.InitEmpty()
	doublePt.SetTimestamp(ts)
	doublePt.SetValue(doubleVal)
	doublePtWithLabels := pdata.NewDoubleDataPoint()
	doublePt.CopyTo(doublePtWithLabels)
	labels.CopyTo(doublePtWithLabels.LabelsMap())

	int64Val := int64(123)
	int64Pt := pdata.NewIntDataPoint()
	int64Pt.InitEmpty()
	int64Pt.SetTimestamp(ts)
	int64Pt.SetValue(int64Val)
	int64PtWithLabels := pdata.NewIntDataPoint()
	int64Pt.CopyTo(int64PtWithLabels)
	labels.CopyTo(int64PtWithLabels.LabelsMap())

	histBounds := []float64{1, 2, 4}
	histCounts := []uint64{4, 2, 3, 7}
	histDP := pdata.NewIntHistogramDataPoint()
	histDP.InitEmpty()
	histDP.SetTimestamp(ts)
	histDP.SetCount(16)
	histDP.SetSum(100)
	histDP.SetExplicitBounds(histBounds)
	histDP.SetBucketCounts(histCounts)
	labels.CopyTo(histDP.LabelsMap())

	doubleHistDP := pdata.NewDoubleHistogramDataPoint()
	doubleHistDP.InitEmpty()
	doubleHistDP.SetTimestamp(ts)
	doubleHistDP.SetCount(16)
	doubleHistDP.SetSum(100.0)
	doubleHistDP.SetExplicitBounds(histBounds)
	doubleHistDP.SetBucketCounts(histCounts)
	labels.CopyTo(doubleHistDP.LabelsMap())

	histDPNoBuckets := pdata.NewIntHistogramDataPoint()
	histDPNoBuckets.InitEmpty()
	histDPNoBuckets.SetCount(2)
	histDPNoBuckets.SetSum(10)
	histDPNoBuckets.SetTimestamp(ts)
	labels.CopyTo(histDPNoBuckets.LabelsMap())

	tests := []struct {
		name              string
		metricsDataFn     func() pdata.ResourceMetrics
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "nil_node_nil_resources_no_dims",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleGauge)
					m.DoubleGauge().InitEmpty()
					m.DoubleGauge().DataPoints().Append(doublePt)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeIntGauge)
					m.IntGauge().InitEmpty()
					m.IntGauge().DataPoints().Append(int64Pt)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("cumulative_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleSum)
					m.DoubleSum().InitEmpty()
					m.DoubleSum().SetIsMonotonic(true)
					m.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
					m.DoubleSum().DataPoints().Append(doublePt)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeIntSum)
					m.IntSum().InitEmpty()
					m.IntSum().SetIsMonotonic(true)
					m.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
					m.IntSum().DataPoints().Append(int64Pt)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("delta_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleSum)
					m.DoubleSum().InitEmpty()
					m.DoubleSum().SetIsMonotonic(true)
					m.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
					m.DoubleSum().DataPoints().Append(doublePt)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("delta_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeIntSum)
					m.IntSum().InitEmpty()
					m.IntSum().SetIsMonotonic(true)
					m.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
					m.IntSum().DataPoints().Append(int64Pt)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_sum_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleSum)
					m.DoubleSum().InitEmpty()
					m.DoubleSum().SetIsMonotonic(false)
					m.DoubleSum().DataPoints().Append(doublePt)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_sum_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeIntSum)
					m.IntSum().InitEmpty()
					m.IntSum().SetIsMonotonic(false)
					m.IntSum().DataPoints().Append(int64Pt)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", tsMSecs, &sfxMetricTypeGauge, nil, doubleVal),
				int64SFxDataPoint("gauge_int_with_dims", tsMSecs, &sfxMetricTypeGauge, nil, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, nil, doubleVal),
				int64SFxDataPoint("cumulative_int_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, nil, int64Val),
				doubleSFxDataPoint("delta_double_with_dims", tsMSecs, &sfxMetricTypeCounter, nil, doubleVal),
				int64SFxDataPoint("delta_int_with_dims", tsMSecs, &sfxMetricTypeCounter, nil, int64Val),
				doubleSFxDataPoint("gauge_sum_double_with_dims", tsMSecs, &sfxMetricTypeGauge, nil, doubleVal),
				int64SFxDataPoint("gauge_sum_int_with_dims", tsMSecs, &sfxMetricTypeGauge, nil, int64Val),
			},
		},
		{
			name: "nil_node_and_resources_with_dims",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleGauge)
					m.DoubleGauge().InitEmpty()
					m.DoubleGauge().DataPoints().Append(doublePtWithLabels)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeIntGauge)
					m.IntGauge().InitEmpty()
					m.IntGauge().DataPoints().Append(int64PtWithLabels)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("cumulative_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleSum)
					m.DoubleSum().InitEmpty()
					m.DoubleSum().SetIsMonotonic(true)
					m.DoubleSum().DataPoints().Append(doublePtWithLabels)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeIntSum)
					m.IntSum().InitEmpty()
					m.IntSum().SetIsMonotonic(true)
					m.IntSum().DataPoints().Append(int64PtWithLabels)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", tsMSecs, &sfxMetricTypeGauge, labelMap, doubleVal),
				int64SFxDataPoint("gauge_int_with_dims", tsMSecs, &sfxMetricTypeGauge, labelMap, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, labelMap, doubleVal),
				int64SFxDataPoint("cumulative_int_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, labelMap, int64Val),
			},
		},
		{
			name: "with_node_resources_dims",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("k/n0", "vn0")
				res.Attributes().InsertString("k/n1", "vn1")

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleGauge)
					m.DoubleGauge().InitEmpty()
					m.DoubleGauge().DataPoints().Append(doublePtWithLabels)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeIntGauge)
					m.IntGauge().InitEmpty()
					m.IntGauge().DataPoints().Append(int64PtWithLabels)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(map[string]string{
						"k_n0": "vn0",
						"k_n1": "vn1",
						"k_r0": "vr0",
						"k_r1": "vr1",
					}, labelMap),
					doubleVal),
				int64SFxDataPoint(
					"gauge_int_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(map[string]string{
						"k_n0": "vn0",
						"k_n1": "vn1",
						"k_r0": "vr0",
						"k_r1": "vr1",
					}, labelMap),
					int64Val),
			},
		},
		{
			name: "with_resources_cloud_partial_aws_dim",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderAWS)
				res.Attributes().InsertString("cloud.account.id", "efgh")
				res.Attributes().InsertString("cloud.region", "us-east")

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleGauge)
					m.DoubleGauge().InitEmpty()
					m.DoubleGauge().DataPoints().Append(doublePtWithLabels)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(labelMap, map[string]string{
						"cloud_account_id": "efgh",
						"cloud_provider":   conventions.AttributeCloudProviderAWS,
						"cloud_region":     "us-east",
						"k_r0":             "vr0",
						"k_r1":             "vr1",
					}),
					doubleVal),
			},
		},
		{
			name: "with_resources_cloud_aws_dim",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderAWS)
				res.Attributes().InsertString("cloud.account.id", "efgh")
				res.Attributes().InsertString("cloud.region", "us-east")
				res.Attributes().InsertString("host.id", "abcd")

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleGauge)
					m.DoubleGauge().InitEmpty()
					m.DoubleGauge().DataPoints().Append(doublePtWithLabels)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(labelMap, map[string]string{
						"AWSUniqueId": "abcd_us-east_efgh",
						"k_r0":        "vr0",
						"k_r1":        "vr1",
					}),
					doubleVal),
			},
		},
		{
			name: "with_resources_cloud_gcp_dim_partial",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderGCP)
				res.Attributes().InsertString("host.id", "abcd")

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleGauge)
					m.DoubleGauge().InitEmpty()
					m.DoubleGauge().DataPoints().Append(doublePtWithLabels)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(labelMap, map[string]string{
						"host_id":        "abcd",
						"cloud_provider": conventions.AttributeCloudProviderGCP,
						"k_r0":           "vr0",
						"k_r1":           "vr1",
					}),
					doubleVal),
			},
		},
		{
			name: "with_resources_cloud_gcp_dim",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderGCP)
				res.Attributes().InsertString("host.id", "abcd")
				res.Attributes().InsertString("cloud.account.id", "efgh")

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeDoubleGauge)
					m.DoubleGauge().InitEmpty()
					m.DoubleGauge().DataPoints().Append(doublePtWithLabels)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(labelMap, map[string]string{
						"gcp_id": "efgh_abcd",
						"k_r0":   "vr0",
						"k_r1":   "vr1",
					}),
					doubleVal),
			},
		},
		{
			name: "histograms",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("int_histo")
					m.SetDataType(pdata.MetricDataTypeIntHistogram)
					m.IntHistogram().InitEmpty()
					m.IntHistogram().DataPoints().Append(histDP)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("double_histo")
					m.SetDataType(pdata.MetricDataTypeDoubleHistogram)
					m.DoubleHistogram().InitEmpty()
					m.DoubleHistogram().DataPoints().Append(doubleHistDP)
					ilm.Metrics().Append(m)
				}

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("int_delta_histo")
					m.SetDataType(pdata.MetricDataTypeIntHistogram)
					m.IntHistogram().InitEmpty()
					m.IntHistogram().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
					m.IntHistogram().DataPoints().Append(histDP)
					ilm.Metrics().Append(m)
				}
				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("double_delta_histo")
					m.SetDataType(pdata.MetricDataTypeDoubleHistogram)
					m.DoubleHistogram().InitEmpty()
					m.DoubleHistogram().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
					m.DoubleHistogram().DataPoints().Append(doubleHistDP)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: mergeDPs(
				expectedFromIntHistogram("int_histo", tsMSecs, labelMap, histDP, false),
				expectedFromDoubleHistogram("double_histo", tsMSecs, labelMap, doubleHistDP, false),
				expectedFromIntHistogram("int_delta_histo", tsMSecs, labelMap, histDP, true),
				expectedFromDoubleHistogram("double_delta_histo", tsMSecs, labelMap, doubleHistDP, true),
			),
		},
		{
			name: "distribution_no_buckets",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				out.InitEmpty()

				out.InstrumentationLibraryMetrics().Resize(1)
				ilm := out.InstrumentationLibraryMetrics().At(0)

				{
					m := pdata.NewMetric()
					m.InitEmpty()
					m.SetName("no_bucket_histo")
					m.SetDataType(pdata.MetricDataTypeIntHistogram)
					m.IntHistogram().InitEmpty()
					m.IntHistogram().DataPoints().Append(histDPNoBuckets)
					ilm.Metrics().Append(m)
				}

				return out
			},
			wantSfxDataPoints: expectedFromIntHistogram("no_bucket_histo", tsMSecs, labelMap, histDPNoBuckets, false),
		},
	}
	c := NewMetricsConverter(logger, nil)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSfxDataPoints := c.MetricDataToSignalFxV2(tt.metricsDataFn())
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.Equal(t, tt.wantSfxDataPoints, gotSfxDataPoints)
		})
	}
}

func TestMetricDataToSignalFxV2WithTranslation(t *testing.T) {
	translator, err := NewMetricTranslator([]Rule{
		{
			Action: ActionRenameDimensionKeys,
			Mapping: map[string]string{
				"old.dim": "new.dim",
			},
		},
	}, 1)
	require.NoError(t, err)

	md := pdata.NewMetric()
	md.InitEmpty()
	md.SetDataType(pdata.MetricDataTypeIntGauge)
	md.IntGauge().InitEmpty()
	md.IntGauge().DataPoints().Resize(1)
	md.SetName("metric1")
	dp := md.IntGauge().DataPoints().At(0)
	dp.SetValue(123)
	dp.LabelsMap().InitFromMap(map[string]string{
		"old.dim": "val1",
	})

	gaugeType := sfxpb.MetricType_GAUGE
	expected := []*sfxpb.DataPoint{
		{
			Metric: "metric1",
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(123),
			},
			MetricType: &gaugeType,
			Dimensions: []*sfxpb.Dimension{
				{
					Key:   "new_dim",
					Value: "val1",
				},
			},
		},
	}
	c := NewMetricsConverter(zap.NewNop(), translator)
	assert.EqualValues(t, expected, c.MetricDataToSignalFxV2(wrapMetric(md)))
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
	ts int64,
	metricType *sfxpb.MetricType,
	dims map[string]string,
	val float64,
) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     metric,
		Timestamp:  ts,
		Value:      sfxpb.Datum{DoubleValue: &val},
		MetricType: metricType,
		Dimensions: sfxDimensions(dims),
	}
}

func int64SFxDataPoint(
	metric string,
	ts int64,
	metricType *sfxpb.MetricType,
	dims map[string]string,
	val int64,
) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     metric,
		Timestamp:  ts,
		Value:      sfxpb.Datum{IntValue: &val},
		MetricType: metricType,
		Dimensions: sfxDimensions(dims),
	}
}

func sfxDimensions(m map[string]string) []*sfxpb.Dimension {
	sfxDims := make([]*sfxpb.Dimension, 0, len(m))
	for k, v := range m {
		sfxDims = append(sfxDims, &sfxpb.Dimension{
			Key:   k,
			Value: v,
		})
	}

	return sfxDims
}

func expectedFromIntHistogram(
	metricName string,
	ts int64,
	dims map[string]string,
	histDP pdata.IntHistogramDataPoint,
	isDelta bool,
) []*sfxpb.DataPoint {
	buckets := histDP.BucketCounts()

	dps := make([]*sfxpb.DataPoint, 0)

	typ := &sfxMetricTypeCumulativeCounter
	if isDelta {
		typ = &sfxMetricTypeCounter
	}

	dps = append(dps,
		int64SFxDataPoint(metricName+"_count", ts, typ, dims,
			int64(histDP.Count())),
		int64SFxDataPoint(metricName, ts, typ, dims,
			histDP.Sum()))

	explicitBounds := histDP.ExplicitBounds()
	if explicitBounds == nil {
		return dps
	}
	for i := 0; i < len(explicitBounds); i++ {
		dimsCopy := util.CloneStringMap(dims)
		dimsCopy[upperBoundDimensionKey] = float64ToDimValue(explicitBounds[i])
		dps = append(dps,
			int64SFxDataPoint(metricName+"_bucket", ts,
				typ, dimsCopy,
				int64(buckets[i])))
	}
	dimsCopy := util.CloneStringMap(dims)
	dimsCopy[upperBoundDimensionKey] = float64ToDimValue(math.Inf(1))
	dps = append(dps,
		int64SFxDataPoint(metricName+"_bucket", ts, typ,
			dimsCopy,
			int64(buckets[len(buckets)-1])))
	return dps
}

func expectedFromDoubleHistogram(
	metricName string,
	ts int64,
	dims map[string]string,
	histDP pdata.DoubleHistogramDataPoint,
	isDelta bool,
) []*sfxpb.DataPoint {
	buckets := histDP.BucketCounts()

	dps := make([]*sfxpb.DataPoint, 0)

	typ := &sfxMetricTypeCumulativeCounter
	if isDelta {
		typ = &sfxMetricTypeCounter
	}

	dps = append(dps,
		int64SFxDataPoint(metricName+"_count", ts, typ, dims,
			int64(histDP.Count())),
		doubleSFxDataPoint(metricName, ts, typ, dims,
			histDP.Sum()))

	explicitBounds := histDP.ExplicitBounds()
	if explicitBounds == nil {
		return dps
	}
	for i := 0; i < len(explicitBounds); i++ {
		dimsCopy := util.CloneStringMap(dims)
		dimsCopy[upperBoundDimensionKey] = float64ToDimValue(explicitBounds[i])
		dps = append(dps,
			int64SFxDataPoint(metricName+"_bucket", ts,
				typ, dimsCopy,
				int64(buckets[i])))
	}
	dimsCopy := util.CloneStringMap(dims)
	dimsCopy[upperBoundDimensionKey] = float64ToDimValue(math.Inf(1))
	dps = append(dps,
		int64SFxDataPoint(metricName+"_bucket", ts, typ,
			dimsCopy,
			int64(buckets[len(buckets)-1])))
	return dps
}

func mergeDPs(dps ...[]*sfxpb.DataPoint) []*sfxpb.DataPoint {
	var out []*sfxpb.DataPoint
	for i := range dps {
		out = append(out, dps[i]...)
	}
	return out
}
