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

package translation

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/util"
)

func Test_MetricDataToSignalFxV2(t *testing.T) {
	logger := zap.NewNop()

	labelMap := map[string]string{
		"k0": "v0",
		"k1": "v1",
	}

	longLabelMap := map[string]string{
		fmt.Sprintf("l%sng_key", strings.Repeat("o", 128)): "v0",
		"k0": "v0",
		"k1": fmt.Sprintf("l%sng_value", strings.Repeat("o", 256)),
		"k2": "v2",
	}

	const unixSecs = int64(1574092046)
	const unixNSecs = int64(11 * time.Millisecond)
	ts := pdata.NewTimestampFromTime(time.Unix(unixSecs, unixNSecs))
	tsMSecs := unixSecs*1e3 + unixNSecs/1e6

	const doubleVal = 1234.5678
	initDoublePt := func(doublePt pdata.NumberDataPoint) {
		doublePt.SetTimestamp(ts)
		doublePt.SetDoubleVal(doubleVal)
	}

	initDoublePtWithLabels := func(doublePtWithLabels pdata.NumberDataPoint) {
		initDoublePt(doublePtWithLabels)
		doublePtWithLabels.Attributes().InitFromMap(stringMapToAttributeMap(labelMap))
	}

	initDoublePtWithLongLabels := func(doublePtWithLabels pdata.NumberDataPoint) {
		initDoublePt(doublePtWithLabels)
		doublePtWithLabels.Attributes().InitFromMap(stringMapToAttributeMap(longLabelMap))
	}

	differentLabelMap := map[string]string{
		"k00": "v00",
		"k11": "v11",
	}
	initDoublePtWithDifferentLabels := func(doublePtWithDifferentLabels pdata.NumberDataPoint) {
		initDoublePt(doublePtWithDifferentLabels)
		doublePtWithDifferentLabels.Attributes().InitFromMap(stringMapToAttributeMap(differentLabelMap))
	}

	const int64Val = int64(123)
	initInt64Pt := func(int64Pt pdata.NumberDataPoint) {
		int64Pt.SetTimestamp(ts)
		int64Pt.SetIntVal(int64Val)
	}

	initInt64PtWithLabels := func(int64PtWithLabels pdata.NumberDataPoint) {
		initInt64Pt(int64PtWithLabels)
		int64PtWithLabels.Attributes().InitFromMap(stringMapToAttributeMap(labelMap))
	}

	histBounds := []float64{1, 2, 4}
	histCounts := []uint64{4, 2, 3, 7}

	initHistDP := func(histDP pdata.HistogramDataPoint) {
		histDP.SetTimestamp(ts)
		histDP.SetCount(16)
		histDP.SetSum(100.0)
		histDP.SetExplicitBounds(histBounds)
		histDP.SetBucketCounts(histCounts)
		histDP.Attributes().InitFromMap(stringMapToAttributeMap(labelMap))
	}
	histDP := pdata.NewHistogramDataPoint()
	initHistDP(histDP)

	initHistDPNoBuckets := func(histDP pdata.HistogramDataPoint) {
		histDP.SetCount(2)
		histDP.SetSum(10)
		histDP.SetTimestamp(ts)
		histDP.Attributes().InitFromMap(stringMapToAttributeMap(labelMap))
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
		summaryDP.Attributes().InitFromMap(stringMapToAttributeMap(labelMap))
	}

	initEmptySummaryDP := func(summaryDP pdata.SummaryDataPoint) {
		summaryDP.SetTimestamp(ts)
		summaryDP.SetSum(summarySumVal)
		summaryDP.SetCount(summaryCountVal)
		summaryDP.Attributes().InitFromMap(stringMapToAttributeMap(labelMap))
	}

	tests := []struct {
		name              string
		metricsDataFn     func() pdata.ResourceMetrics
		excludeMetrics    []dpfilters.MetricFilter
		includeMetrics    []dpfilters.MetricFilter
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "nil_node_nil_resources_no_dims",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()

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
					m.Sum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
					initInt64Pt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("delta_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
					initDoublePt(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("delta_int_with_dims")
					m.SetDataType(pdata.MetricDataTypeSum)
					m.Sum().SetIsMonotonic(true)
					m.Sum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
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
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()

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
				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("k/n0", "vn0")
				res.Attributes().InsertString("k/n1", "vn1")

				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
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
			name: "with_node_resources_dims - long metric name",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("k/n0", "vn0")
				res.Attributes().InsertString("k/n1", "vn1")

				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				ilm.Metrics().EnsureCapacity(5)

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName(fmt.Sprintf("l%sng_name", strings.Repeat("o", 256)))
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
					m.SetName(fmt.Sprintf("l%sng_name", strings.Repeat("o", 256)))
					m.SetDataType(pdata.MetricDataTypeGauge)
					initDoublePtWithLabels(m.Gauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName(fmt.Sprintf("l%sng_name", strings.Repeat("o", 256)))
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
			name: "with_node_resources_dims - long dimension name/value",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("k/n0", "vn0")
				res.Attributes().InsertString("k/n1", "vn1")

				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				ilm.Metrics().EnsureCapacity(1)

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					m.SetDataType(pdata.MetricDataTypeGauge)
					initDoublePtWithLongLabels(m.Gauge().DataPoints().AppendEmpty())
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
					}, map[string]string{
						"k0": "v0",
						"k2": "v2",
					}),
					doubleVal),
			},
		},
		{
			name: "with_resources_cloud_partial_aws_dim",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderAWS)
				res.Attributes().InsertString("cloud.account.id", "efgh")
				res.Attributes().InsertString("cloud.region", "us-east")

				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				m.SetDataType(pdata.MetricDataTypeGauge)
				initDoublePtWithLabels(m.Gauge().DataPoints().AppendEmpty())

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
				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderAWS)
				res.Attributes().InsertString("cloud.account.id", "efgh")
				res.Attributes().InsertString("cloud.region", "us-east")
				res.Attributes().InsertString("host.id", "abcd")

				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				m.SetDataType(pdata.MetricDataTypeGauge)
				initDoublePtWithLabels(m.Gauge().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(labelMap, map[string]string{
						"cloud_provider":   conventions.AttributeCloudProviderAWS,
						"cloud_account_id": "efgh",
						"cloud_region":     "us-east",
						"host_id":          "abcd",
						"AWSUniqueId":      "abcd_us-east_efgh",
						"k_r0":             "vr0",
						"k_r1":             "vr1",
					}),
					doubleVal),
			},
		},
		{
			name: "with_resources_cloud_gcp_dim_partial",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderGCP)
				res.Attributes().InsertString("host.id", "abcd")

				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				m.SetDataType(pdata.MetricDataTypeGauge)
				initDoublePtWithLabels(m.Gauge().DataPoints().AppendEmpty())

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
				res := out.Resource()
				res.Attributes().InsertString("k/r0", "vr0")
				res.Attributes().InsertString("k/r1", "vr1")
				res.Attributes().InsertString("cloud.provider", conventions.AttributeCloudProviderGCP)
				res.Attributes().InsertString("host.id", "abcd")
				res.Attributes().InsertString("cloud.account.id", "efgh")

				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				m.SetDataType(pdata.MetricDataTypeGauge)
				initDoublePtWithLabels(m.Gauge().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					tsMSecs,
					&sfxMetricTypeGauge,
					util.MergeStringMaps(labelMap, map[string]string{
						"gcp_id":           "efgh_abcd",
						"k_r0":             "vr0",
						"k_r1":             "vr1",
						"cloud_provider":   conventions.AttributeCloudProviderGCP,
						"host_id":          "abcd",
						"cloud_account_id": "efgh",
					}),
					doubleVal),
			},
		},
		{
			name: "histograms",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
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
					m.Histogram().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
					initHistDP(m.Histogram().DataPoints().AppendEmpty())
				}
				return out
			},
			wantSfxDataPoints: mergeDPs(
				expectedFromHistogram("double_histo", tsMSecs, labelMap, histDP, false),
				expectedFromHistogram("double_delta_histo", tsMSecs, labelMap, histDP, true),
			),
		},
		{
			name: "distribution_no_buckets",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("no_bucket_histo")
				m.SetDataType(pdata.MetricDataTypeHistogram)
				initHistDPNoBuckets(m.Histogram().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: expectedFromHistogram("no_bucket_histo", tsMSecs, labelMap, histDPNoBuckets, false),
		},
		{
			name: "summaries",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("summary")
				m.SetDataType(pdata.MetricDataTypeSummary)
				initSummaryDP(m.Summary().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: expectedFromSummary("summary", tsMSecs, labelMap, summaryCountVal, summarySumVal),
		},
		{
			name: "empty_summary",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("empty_summary")
				m.SetDataType(pdata.MetricDataTypeSummary)
				initEmptySummaryDP(m.Summary().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: expectedFromEmptySummary("empty_summary", tsMSecs, labelMap, summaryCountVal, summarySumVal),
		},
		{
			name: "with_exclude_metrics_filter",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()

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
					initDoublePtWithDifferentLabels(m.Sum().DataPoints().AppendEmpty())
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
			excludeMetrics: []dpfilters.MetricFilter{
				{
					MetricNames: []string{"gauge_double_with_dims"},
				},
				{
					MetricName: "cumulative_int_with_dims",
				},
				{
					MetricName: "gauge_int_with_dims",
					Dimensions: map[string]interface{}{
						"k0": []interface{}{"v1"},
					},
				},
				{
					MetricName: "cumulative_double_with_dims",
					Dimensions: map[string]interface{}{
						"k0": []interface{}{"v0"},
					},
				},
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("gauge_int_with_dims", tsMSecs, &sfxMetricTypeGauge, labelMap, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, differentLabelMap, doubleVal),
			},
		},
		{
			// To validate that filters in include serves as override to the ones in exclude list.
			name: "with_include_and_exclude_metrics_filter",
			metricsDataFn: func() pdata.ResourceMetrics {
				out := pdata.NewResourceMetrics()
				ilm := out.InstrumentationLibraryMetrics().AppendEmpty()

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
					initDoublePtWithDifferentLabels(m.Sum().DataPoints().AppendEmpty())
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
			excludeMetrics: []dpfilters.MetricFilter{
				{
					MetricNames: []string{"gauge_double_with_dims"},
				},
				{
					MetricName: "cumulative_int_with_dims",
				},
				{
					MetricName: "gauge_int_with_dims",
					Dimensions: map[string]interface{}{
						"k0": []interface{}{"v1"},
					},
				},
				{
					MetricName: "cumulative_double_with_dims",
					Dimensions: map[string]interface{}{
						"k0": []interface{}{"v0"},
					},
				},
			},
			includeMetrics: []dpfilters.MetricFilter{
				{
					MetricName: "cumulative_int_with_dims",
				},
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("gauge_int_with_dims", tsMSecs, &sfxMetricTypeGauge, labelMap, int64Val),
				doubleSFxDataPoint("cumulative_double_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, differentLabelMap, doubleVal),
				int64SFxDataPoint("cumulative_int_with_dims", tsMSecs, &sfxMetricTypeCumulativeCounter, labelMap, int64Val),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewMetricsConverter(logger, nil, tt.excludeMetrics, tt.includeMetrics, "")
			require.NoError(t, err)
			md := tt.metricsDataFn()
			gotSfxDataPoints := c.MetricDataToSignalFxV2(md)
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

	rm := pdata.NewResourceMetrics()
	md := rm.InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	md.SetDataType(pdata.MetricDataTypeGauge)
	md.SetName("metric1")
	dp := md.Gauge().DataPoints().AppendEmpty()
	dp.SetIntVal(123)
	dp.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"old.dim": pdata.NewAttributeValueString("val1"),
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
	c, err := NewMetricsConverter(zap.NewNop(), translator, nil, nil, "")
	require.NoError(t, err)
	assert.EqualValues(t, expected, c.MetricDataToSignalFxV2(rm))
}

func TestDimensionKeyCharsWithPeriod(t *testing.T) {
	translator, err := NewMetricTranslator([]Rule{
		{
			Action: ActionRenameDimensionKeys,
			Mapping: map[string]string{
				"old.dim.with.periods": "new.dim.with.periods",
			},
		},
	}, 1)
	require.NoError(t, err)

	rm := pdata.NewResourceMetrics()
	md := rm.InstrumentationLibraryMetrics().AppendEmpty().Metrics().AppendEmpty()
	md.SetDataType(pdata.MetricDataTypeGauge)
	md.SetName("metric1")
	dp := md.Gauge().DataPoints().AppendEmpty()
	dp.SetIntVal(123)
	dp.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"old.dim.with.periods": pdata.NewAttributeValueString("val1"),
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
					Key:   "new.dim.with.periods",
					Value: "val1",
				},
			},
		},
	}
	c, err := NewMetricsConverter(zap.NewNop(), translator, nil, nil, "_-.")
	require.NoError(t, err)
	assert.EqualValues(t, expected, c.MetricDataToSignalFxV2(rm))

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

func expectedFromHistogram(
	metricName string,
	ts int64,
	dims map[string]string,
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

func expectedFromSummary(name string, ts int64, labelMap map[string]string, count int64, sumVal float64) []*sfxpb.DataPoint {
	countName := name + "_count"
	countPt := int64SFxDataPoint(countName, ts, &sfxMetricTypeCumulativeCounter, labelMap, count)
	sumPt := doubleSFxDataPoint(name, ts, &sfxMetricTypeCumulativeCounter, labelMap, sumVal)
	out := []*sfxpb.DataPoint{countPt, sumPt}
	quantileDimVals := []string{"0.25", "0.5", "0.75", "1"}
	for i := 0; i < 4; i++ {
		qDims := map[string]string{"quantile": quantileDimVals[i]}
		qPt := doubleSFxDataPoint(
			name+"_quantile",
			ts,
			&sfxMetricTypeGauge,
			util.MergeStringMaps(labelMap, qDims),
			float64(i),
		)
		out = append(out, qPt)
	}
	return out
}

func expectedFromEmptySummary(name string, ts int64, labelMap map[string]string, count int64, sumVal float64) []*sfxpb.DataPoint {
	countName := name + "_count"
	countPt := int64SFxDataPoint(countName, ts, &sfxMetricTypeCumulativeCounter, labelMap, count)
	sumPt := doubleSFxDataPoint(name, ts, &sfxMetricTypeCumulativeCounter, labelMap, sumVal)
	return []*sfxpb.DataPoint{countPt, sumPt}
}

func mergeDPs(dps ...[]*sfxpb.DataPoint) []*sfxpb.DataPoint {
	var out []*sfxpb.DataPoint
	for i := range dps {
		out = append(out, dps[i]...)
	}
	return out
}

func TestNewMetricsConverter(t *testing.T) {
	tests := []struct {
		name     string
		excludes []dpfilters.MetricFilter
		want     *MetricsConverter
		wantErr  bool
	}{
		{
			name:     "Error on creating filterSet",
			excludes: []dpfilters.MetricFilter{{}},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMetricsConverter(zap.NewNop(), nil, tt.excludes, nil, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMetricsConverter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMetricsConverter() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetricsConverter_ConvertDimension(t *testing.T) {
	type fields struct {
		metricTranslator        *MetricTranslator
		nonAlphanumericDimChars string
	}
	type args struct {
		dim string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "No translations",
			fields: fields{
				metricTranslator:        nil,
				nonAlphanumericDimChars: "_-",
			},
			args: args{
				dim: "d.i.m",
			},
			want: "d_i_m",
		},
		{
			name: "With translations",
			fields: fields{
				metricTranslator: func() *MetricTranslator {
					t, _ := NewMetricTranslator([]Rule{
						{
							Action: ActionRenameDimensionKeys,
							Mapping: map[string]string{
								"d.i.m": "di.m",
							},
						},
					}, 0)
					return t
				}(),
				nonAlphanumericDimChars: "_-",
			},
			args: args{
				dim: "d.i.m",
			},
			want: "di_m",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewMetricsConverter(zap.NewNop(), tt.fields.metricTranslator, nil, nil, tt.fields.nonAlphanumericDimChars)
			require.NoError(t, err)
			if got := c.ConvertDimension(tt.args.dim); got != tt.want {
				t.Errorf("ConvertDimension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertSummary(t *testing.T) {
	extraDims := []*sfxpb.Dimension{{
		Key:   "dim1",
		Value: "val1",
	}}
	summarys := pdata.NewSummaryDataPointSlice()
	summary := summarys.AppendEmpty()
	const count = 42
	summary.SetCount(count)
	const sum = 10.0
	summary.SetSum(sum)
	const startTime = 55 * 1e6
	summary.SetStartTimestamp(pdata.Timestamp(startTime))
	timestamp := 111 * 1e6
	summary.SetTimestamp(pdata.Timestamp(timestamp))
	qvs := summary.QuantileValues()
	for i := 0; i < 4; i++ {
		qv := qvs.AppendEmpty()
		qv.SetQuantile(0.25 * float64(i+1))
		qv.SetValue(float64(i))
	}
	dps := convertSummaryDataPoints(summarys, "metric_name", extraDims)

	pt := dps[0]
	assert.Equal(t, sfxpb.MetricType_CUMULATIVE_COUNTER, *pt.MetricType)
	assert.Equal(t, int64(111), pt.Timestamp)
	assert.Equal(t, "metric_name_count", pt.Metric)
	assert.Equal(t, int64(count), pt.Value.GetIntValue())
	assert.Equal(t, 1, len(pt.Dimensions))
	assertHasExtraDim(t, pt)

	pt = dps[1]
	assert.Equal(t, sfxpb.MetricType_CUMULATIVE_COUNTER, *pt.MetricType)
	assert.Equal(t, int64(111), pt.Timestamp)
	assert.Equal(t, "metric_name", pt.Metric)
	assert.Equal(t, sum, pt.Value.GetDoubleValue())
	assert.Equal(t, 1, len(pt.Dimensions))
	assertHasExtraDim(t, pt)

	pt = dps[2]
	assert.Equal(t, sfxpb.MetricType_GAUGE, *pt.MetricType)
	assert.Equal(t, int64(111), pt.Timestamp)
	assert.Equal(t, "metric_name_quantile", pt.Metric)
	assert.Equal(t, 0.0, pt.Value.GetDoubleValue())
	assert.Equal(t, 2, len(pt.Dimensions))
	dim := pt.Dimensions[1]
	assert.Equal(t, "quantile", dim.Key)

	for i := 0; i < 4; i++ {
		pt = dps[i+2]
		assert.Equal(t, sfxpb.MetricType_GAUGE, *pt.MetricType)
		assert.Equal(t, int64(111), pt.Timestamp)
		assert.Equal(t, "metric_name_quantile", pt.Metric)
		assert.EqualValues(t, i, pt.Value.GetDoubleValue())
		assert.Equal(t, 2, len(pt.Dimensions))
		dim = pt.Dimensions[1]
		assert.Equal(t, "quantile", dim.Key)
	}

	assert.Equal(t, "0.25", dps[2].Dimensions[1].Value)
	assert.Equal(t, "0.5", dps[3].Dimensions[1].Value)
	assert.Equal(t, "0.75", dps[4].Dimensions[1].Value)
	assert.Equal(t, "1", dps[5].Dimensions[1].Value)

	println()
}

func assertHasExtraDim(t *testing.T, pt *sfxpb.DataPoint) {
	extraDim := pt.Dimensions[0]
	assert.Equal(t, "dim1", extraDim.Key)
	assert.Equal(t, "val1", extraDim.Value)
}

func stringMapToAttributeMap(m map[string]string) map[string]pdata.AttributeValue {
	ret := map[string]pdata.AttributeValue{}
	for k, v := range m {
		ret[k] = pdata.NewAttributeValueString(v)
	}
	return ret
}
