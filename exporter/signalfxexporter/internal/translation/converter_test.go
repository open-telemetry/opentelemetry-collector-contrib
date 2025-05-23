// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/maps"
)

const (
	unixSecs  = int64(1574092046)
	unixNSecs = int64(11 * time.Millisecond)
	tsMSecs   = unixSecs*1e3 + unixNSecs/1e6
)

// Not const to be able to take the address of them.
var (
	doubleVal = 1234.5678
	int64Val  = int64(123)
)

func Test_MetricDataToSignalFxV2(t *testing.T) {
	logger := zap.NewNop()

	labelMap := map[string]any{
		"k0": "v0",
		"k1": "v1",
	}

	longLabelMap := map[string]any{
		fmt.Sprintf("l%sng_key", strings.Repeat("o", 128)): "v0",
		"k0": "v0",
		"k1": fmt.Sprintf("l%sng_value", strings.Repeat("o", 256)),
		"k2": "v2",
	}

	ts := pcommon.NewTimestampFromTime(time.Unix(unixSecs, unixNSecs))

	initDoublePt := func(doublePt pmetric.NumberDataPoint) {
		doublePt.SetTimestamp(ts)
		doublePt.SetDoubleValue(doubleVal)
	}

	initDoublePtWithLabels := func(doublePtWithLabels pmetric.NumberDataPoint) {
		initDoublePt(doublePtWithLabels)
		assert.NoError(t, doublePtWithLabels.Attributes().FromRaw(labelMap))
	}

	initDoublePtWithLongLabels := func(doublePtWithLabels pmetric.NumberDataPoint) {
		initDoublePt(doublePtWithLabels)
		assert.NoError(t, doublePtWithLabels.Attributes().FromRaw(longLabelMap))
	}

	differentLabelMap := map[string]any{
		"k00": "v00",
		"k11": "v11",
	}
	initDoublePtWithDifferentLabels := func(doublePtWithDifferentLabels pmetric.NumberDataPoint) {
		initDoublePt(doublePtWithDifferentLabels)
		assert.NoError(t, doublePtWithDifferentLabels.Attributes().FromRaw(differentLabelMap))
	}

	initInt64Pt := func(int64Pt pmetric.NumberDataPoint) {
		int64Pt.SetTimestamp(ts)
		int64Pt.SetIntValue(int64Val)
	}

	initInt64PtWithLabels := func(int64PtWithLabels pmetric.NumberDataPoint) {
		initInt64Pt(int64PtWithLabels)
		assert.NoError(t, int64PtWithLabels.Attributes().FromRaw(labelMap))
	}

	tests := []struct {
		name              string
		metricsFn         func() pmetric.Metrics
		excludeMetrics    []dpfilters.MetricFilter
		includeMetrics    []dpfilters.MetricFilter
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "nil_node_nil_resources_no_dims",
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
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, nil),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, nil),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, nil),
				doubleSFxDataPoint("delta_double_with_dims", &sfxMetricTypeCounter, nil),
				int64SFxDataPoint("delta_int_with_dims", &sfxMetricTypeCounter, nil),
				doubleSFxDataPoint("gauge_sum_double_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("gauge_sum_int_with_dims", &sfxMetricTypeGauge, nil),
			},
		},
		{
			name: "nil_node_and_resources_with_dims",
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
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, labelMap),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
			},
		},
		{
			name: "with_node_resources_dims",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k/r0", "vr0")
				res.Attributes().PutStr("k/r1", "vr1")
				res.Attributes().PutStr("k/n0", "vn0")
				res.Attributes().PutStr("k/n1", "vn1")

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
					maps.MergeRawMaps(map[string]any{
						"k_n0": "vn0",
						"k_n1": "vn1",
						"k_r0": "vr0",
						"k_r1": "vr1",
					}, labelMap)),
				int64SFxDataPoint(
					"gauge_int_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(map[string]any{
						"k_n0": "vn0",
						"k_n1": "vn1",
						"k_r0": "vr0",
						"k_r1": "vr1",
					}, labelMap)),
			},
		},
		{
			name: "with_node_resources_dims - long metric name",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k/r0", "vr0")
				res.Attributes().PutStr("k/r1", "vr1")
				res.Attributes().PutStr("k/n0", "vn0")
				res.Attributes().PutStr("k/n1", "vn1")

				ilm := rm.ScopeMetrics().AppendEmpty()
				ilm.Metrics().EnsureCapacity(5)

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName(fmt.Sprintf("l%sng_name", strings.Repeat("o", 256)))
					initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_int_with_dims")
					initInt64PtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName(fmt.Sprintf("l%sng_name", strings.Repeat("o", 256)))
					initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName(fmt.Sprintf("l%sng_name", strings.Repeat("o", 256)))
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
				int64SFxDataPoint(
					"gauge_int_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(map[string]any{
						"k_n0": "vn0",
						"k_n1": "vn1",
						"k_r0": "vr0",
						"k_r1": "vr1",
					}, labelMap)),
				int64SFxDataPoint(
					"gauge_int_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(map[string]any{
						"k_n0": "vn0",
						"k_n1": "vn1",
						"k_r0": "vr0",
						"k_r1": "vr1",
					}, labelMap)),
			},
		},
		{
			name: "with_node_resources_dims - long dimension name/value",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k/r0", "vr0")
				res.Attributes().PutStr("k/r1", "vr1")
				res.Attributes().PutStr("k/n0", "vn0")
				res.Attributes().PutStr("k/n1", "vn1")

				ilm := rm.ScopeMetrics().AppendEmpty()
				ilm.Metrics().EnsureCapacity(1)

				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("gauge_double_with_dims")
					initDoublePtWithLongLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(map[string]any{
						"k_n0": "vn0",
						"k_n1": "vn1",
						"k_r0": "vr0",
						"k_r1": "vr1",
					}, map[string]any{
						"k0": "v0",
						"k2": "v2",
					})),
			},
		},
		{
			name: "with_resources_cloud_partial_aws_dim",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k/r0", "vr0")
				res.Attributes().PutStr("k/r1", "vr1")
				res.Attributes().PutStr("cloud.provider", conventions.CloudProviderAWS.Value.AsString())
				res.Attributes().PutStr("cloud.account.id", "efgh")
				res.Attributes().PutStr("cloud.region", "us-east")

				ilm := rm.ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(labelMap, map[string]any{
						"cloud_account_id": "efgh",
						"cloud_provider":   conventions.CloudProviderAWS.Value.AsString(),
						"cloud_region":     "us-east",
						"k_r0":             "vr0",
						"k_r1":             "vr1",
					})),
			},
		},
		{
			name: "with_resources_cloud_aws_dim",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k/r0", "vr0")
				res.Attributes().PutStr("k/r1", "vr1")
				res.Attributes().PutStr("cloud.provider", conventions.CloudProviderAWS.Value.AsString())
				res.Attributes().PutStr("cloud.account.id", "efgh")
				res.Attributes().PutStr("cloud.region", "us-east")
				res.Attributes().PutStr("host.id", "abcd")

				ilm := rm.ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(labelMap, map[string]any{
						"cloud_provider":   conventions.CloudProviderAWS.Value.AsString(),
						"cloud_account_id": "efgh",
						"cloud_region":     "us-east",
						"host_id":          "abcd",
						"AWSUniqueId":      "abcd_us-east_efgh",
						"k_r0":             "vr0",
						"k_r1":             "vr1",
					})),
			},
		},
		{
			name: "with_resources_cloud_gcp_dim_partial",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k/r0", "vr0")
				res.Attributes().PutStr("k/r1", "vr1")
				res.Attributes().PutStr("cloud.provider", conventions.CloudProviderGCP.Value.AsString())
				res.Attributes().PutStr("host.id", "abcd")

				ilm := rm.ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(labelMap, map[string]any{
						"host_id":        "abcd",
						"cloud_provider": conventions.CloudProviderGCP.Value.AsString(),
						"k_r0":           "vr0",
						"k_r1":           "vr1",
					})),
			},
		},
		{
			name: "with_resources_cloud_gcp_dim",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				rm := out.ResourceMetrics().AppendEmpty()
				res := rm.Resource()
				res.Attributes().PutStr("k/r0", "vr0")
				res.Attributes().PutStr("k/r1", "vr1")
				res.Attributes().PutStr("cloud.provider", conventions.CloudProviderGCP.Value.AsString())
				res.Attributes().PutStr("host.id", "abcd")
				res.Attributes().PutStr("cloud.account.id", "efgh")

				ilm := rm.ScopeMetrics().AppendEmpty()
				m := ilm.Metrics().AppendEmpty()
				m.SetName("gauge_double_with_dims")
				initDoublePtWithLabels(m.SetEmptyGauge().DataPoints().AppendEmpty())

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint(
					"gauge_double_with_dims",
					&sfxMetricTypeGauge,
					maps.MergeRawMaps(labelMap, map[string]any{
						"gcp_id":           "efgh_abcd",
						"k_r0":             "vr0",
						"k_r1":             "vr1",
						"cloud_provider":   conventions.CloudProviderGCP.Value.AsString(),
						"host_id":          "abcd",
						"cloud_account_id": "efgh",
					})),
			},
		},
		{
			name: "with_exclude_metrics_filter",
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
					initDoublePtWithDifferentLabels(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
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
					Dimensions: map[string]any{
						"k0": []any{"v1"},
					},
				},
				{
					MetricName: "cumulative_double_with_dims",
					Dimensions: map[string]any{
						"k0": []any{"v0"},
					},
				},
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, differentLabelMap),
			},
		},
		{
			// To validate that filters in include serves as override to the ones in exclude list.
			name: "with_include_and_exclude_metrics_filter",
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
					initDoublePtWithDifferentLabels(m.Sum().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("cumulative_int_with_dims")
					m.SetEmptySum().SetIsMonotonic(true)
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
					Dimensions: map[string]any{
						"k0": []any{"v1"},
					},
				},
				{
					MetricName: "cumulative_double_with_dims",
					Dimensions: map[string]any{
						"k0": []any{"v0"},
					},
				},
			},
			includeMetrics: []dpfilters.MetricFilter{
				{
					MetricName: "cumulative_int_with_dims",
				},
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, differentLabelMap),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewMetricsConverter(logger, nil, tt.excludeMetrics, tt.includeMetrics, "", true, true)
			require.NoError(t, err)
			md := tt.metricsFn()
			gotSfxDataPoints := c.MetricsToSignalFxV2(md)
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.Equal(t, tt.wantSfxDataPoints, gotSfxDataPoints)
		})
	}
}

func Test_MetricDataToSignalFxV2WithHistogramBuckets(t *testing.T) {
	logger := zap.NewNop()

	labelMap := map[string]any{
		"k0": "v0",
		"k1": "v1",
	}

	labelMapBucket1 := map[string]any{
		"k0": "v0",
		"k1": "v1",
		"le": "1",
	}
	labelMapBucket2 := map[string]any{
		"k0": "v0",
		"k1": "v1",
		"le": "2",
	}
	labelMapBucket3 := map[string]any{
		"k0": "v0",
		"k1": "v1",
		"le": "4",
	}
	labelMapBucket4 := map[string]any{
		"k0": "v0",
		"k1": "v1",
		"le": "+Inf",
	}

	ts := pcommon.NewTimestampFromTime(time.Unix(unixSecs, unixNSecs))

	initDoublePt := func(doublePt pmetric.NumberDataPoint) {
		doublePt.SetTimestamp(ts)
		doublePt.SetDoubleValue(doubleVal)
	}

	initDoublePtWithLabels := func(doublePtWithLabels pmetric.NumberDataPoint) {
		initDoublePt(doublePtWithLabels)
		assert.NoError(t, doublePtWithLabels.Attributes().FromRaw(labelMap))
	}
	initInt64Pt := func(int64Pt pmetric.NumberDataPoint) {
		int64Pt.SetTimestamp(ts)
		int64Pt.SetIntValue(int64Val)
	}

	initInt64PtWithLabels := func(int64PtWithLabels pmetric.NumberDataPoint) {
		initInt64Pt(int64PtWithLabels)
		assert.NoError(t, int64PtWithLabels.Attributes().FromRaw(labelMap))
	}

	initHistDP := func(histDP pmetric.HistogramDataPoint) {
		histDP.SetTimestamp(ts)
		histDP.SetCount(uint64(int64Val))
		histDP.SetSum(doubleVal)
		histDP.ExplicitBounds().FromRaw([]float64{1, 2, 4})
		histDP.BucketCounts().FromRaw([]uint64{4, 2, 3, 7})
		assert.NoError(t, histDP.Attributes().FromRaw(labelMap))
	}
	histDP := pmetric.NewHistogramDataPoint()
	initHistDP(histDP)

	initHistDPNoBuckets := func(histDP pmetric.HistogramDataPoint) {
		histDP.SetCount(uint64(int64Val))
		histDP.SetSum(doubleVal)
		histDP.SetTimestamp(ts)
		assert.NoError(t, histDP.Attributes().FromRaw(labelMap))
	}
	histDPNoBuckets := pmetric.NewHistogramDataPoint()
	initHistDPNoBuckets(histDPNoBuckets)

	initHistPt := func(histDP pmetric.HistogramDataPoint) {
		histDP.SetCount(uint64(int64Val))
		histDP.SetTimestamp(ts)
	}

	tests := []struct {
		name              string
		metricsFn         func() pmetric.Metrics
		excludeMetrics    []dpfilters.MetricFilter
		includeMetrics    []dpfilters.MetricFilter
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "nil_node_nil_resources_no_dims",
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
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_empty_metric")
					initHistPt(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, nil),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, nil),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, nil),
				doubleSFxDataPoint("delta_double_with_dims", &sfxMetricTypeCounter, nil),
				int64SFxDataPoint("delta_int_with_dims", &sfxMetricTypeCounter, nil),
				doubleSFxDataPoint("gauge_sum_double_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("gauge_sum_int_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("histo_empty_metric_count", &sfxMetricTypeCumulativeCounter, nil),
			},
		},
		{
			name: "nil_node_and_resources_with_dims",
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
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_with_no_buckets_dims")
					initHistDPNoBuckets(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_with_buckets_dims")
					initHistDP(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, labelMap),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
				int64SFxDataPoint("histo_with_no_buckets_dims_count", &sfxMetricTypeCumulativeCounter, labelMap),
				doubleSFxDataPoint("histo_with_no_buckets_dims_sum", &sfxMetricTypeCumulativeCounter, labelMap),
				int64SFxDataPoint("histo_with_buckets_dims_count", &sfxMetricTypeCumulativeCounter, labelMap),
				doubleSFxDataPoint("histo_with_buckets_dims_sum", &sfxMetricTypeCumulativeCounter, labelMap),
				histoValue(int64SFxDataPoint("histo_with_buckets_dims_bucket", &sfxMetricTypeCumulativeCounter, labelMapBucket1), 4),
				histoValue(int64SFxDataPoint("histo_with_buckets_dims_bucket", &sfxMetricTypeCumulativeCounter, labelMapBucket2), 6),
				histoValue(int64SFxDataPoint("histo_with_buckets_dims_bucket", &sfxMetricTypeCumulativeCounter, labelMapBucket3), 9),
				histoValue(int64SFxDataPoint("histo_with_buckets_dims_bucket", &sfxMetricTypeCumulativeCounter, labelMapBucket4), 16),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewMetricsConverter(logger, nil, tt.excludeMetrics, tt.includeMetrics, "", false, true)
			require.NoError(t, err)
			md := tt.metricsFn()
			gotSfxDataPoints := c.MetricsToSignalFxV2(md)
			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.Equal(t, tt.wantSfxDataPoints, gotSfxDataPoints)
		})
	}

	testsWithDropHistogramBuckets := []struct {
		name              string
		metricsFn         func() pmetric.Metrics
		excludeMetrics    []dpfilters.MetricFilter
		includeMetrics    []dpfilters.MetricFilter
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "nil_node_nil_resources_no_dims",
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
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_empty_metric")
					initHistPt(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, nil),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, nil),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, nil),
				doubleSFxDataPoint("delta_double_with_dims", &sfxMetricTypeCounter, nil),
				int64SFxDataPoint("delta_int_with_dims", &sfxMetricTypeCounter, nil),
				doubleSFxDataPoint("gauge_sum_double_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("gauge_sum_int_with_dims", &sfxMetricTypeGauge, nil),
				int64SFxDataPoint("histo_empty_metric_count", &sfxMetricTypeCumulativeCounter, nil),
			},
		},
		{
			name: "nil_node_and_resources_with_dims",
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
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_with_no_buckets_dims")
					initHistDPNoBuckets(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_with_buckets_dims")
					initHistDP(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}

				return out
			},
			wantSfxDataPoints: []*sfxpb.DataPoint{
				doubleSFxDataPoint("gauge_double_with_dims", &sfxMetricTypeGauge, labelMap),
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
				int64SFxDataPoint("cumulative_int_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
				int64SFxDataPoint("histo_with_no_buckets_dims_count", &sfxMetricTypeCumulativeCounter, labelMap),
				doubleSFxDataPoint("histo_with_no_buckets_dims_sum", &sfxMetricTypeCumulativeCounter, labelMap),
				int64SFxDataPoint("histo_with_buckets_dims_count", &sfxMetricTypeCumulativeCounter, labelMap),
				doubleSFxDataPoint("histo_with_buckets_dims_sum", &sfxMetricTypeCumulativeCounter, labelMap),
			},
		},
	}

	for _, tt := range testsWithDropHistogramBuckets {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewMetricsConverter(logger, nil, tt.excludeMetrics, tt.includeMetrics, "", true, true)
			require.NoError(t, err)
			md := tt.metricsFn()
			gotSfxDataPoints := c.MetricsToSignalFxV2(md)

			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.Equal(t, tt.wantSfxDataPoints, gotSfxDataPoints)
		})
	}

	testsWithProcessHistogramsFalse := []struct {
		name              string
		metricsFn         func() pmetric.Metrics
		excludeMetrics    []dpfilters.MetricFilter
		includeMetrics    []dpfilters.MetricFilter
		wantCount         int
		wantSfxDataPoints []*sfxpb.DataPoint
	}{
		{
			name: "no_histograms",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
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
				return out
			},
			wantCount: 2,
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
			},
		},
		{
			name: "only_histograms",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_with_buckets")
					initHistDP(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_with_buckets_2")
					initHistDP(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				return out
			},
			wantCount:         0,
			wantSfxDataPoints: []*sfxpb.DataPoint(nil),
		},
		{
			name: "mixed_with_histograms",
			metricsFn: func() pmetric.Metrics {
				out := pmetric.NewMetrics()
				ilm := out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
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
					m.SetName("histo_with_no_buckets")
					initHistDPNoBuckets(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				{
					m := ilm.Metrics().AppendEmpty()
					m.SetName("histo_with_buckets")
					initHistDP(m.SetEmptyHistogram().DataPoints().AppendEmpty())
				}
				return out
			},
			wantCount: 2,
			wantSfxDataPoints: []*sfxpb.DataPoint{
				int64SFxDataPoint("gauge_int_with_dims", &sfxMetricTypeGauge, labelMap),
				doubleSFxDataPoint("cumulative_double_with_dims", &sfxMetricTypeCumulativeCounter, labelMap),
			},
		},
	}

	for _, tt := range testsWithProcessHistogramsFalse {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewMetricsConverter(logger, nil, tt.excludeMetrics, tt.includeMetrics, "", true, false)
			require.NoError(t, err)
			md := tt.metricsFn()
			gotSfxDataPoints := c.MetricsToSignalFxV2(md)

			// Sort SFx dimensions since they are built from maps and the order
			// of those is not deterministic.
			sortDimensions(tt.wantSfxDataPoints)
			sortDimensions(gotSfxDataPoints)
			assert.Len(t, gotSfxDataPoints, tt.wantCount)
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
	}, 1, make(chan struct{}))
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("metric1")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(123)
	dp.Attributes().PutStr("old.dim", "val1")

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
	c, err := NewMetricsConverter(zap.NewNop(), translator, nil, nil, "", false, true)
	require.NoError(t, err)
	assert.Equal(t, expected, c.MetricsToSignalFxV2(md))
}

func TestDimensionKeyCharsWithPeriod(t *testing.T) {
	translator, err := NewMetricTranslator([]Rule{
		{
			Action: ActionRenameDimensionKeys,
			Mapping: map[string]string{
				"old.dim.with.periods": "new.dim.with.periods",
			},
		},
	}, 1, make(chan struct{}))
	require.NoError(t, err)

	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("metric1")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(123)
	dp.Attributes().PutStr("old.dim.with.periods", "val1")

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
	c, err := NewMetricsConverter(zap.NewNop(), translator, nil, nil, "_-.", false, true)
	require.NoError(t, err)
	assert.Equal(t, expected, c.MetricsToSignalFxV2(md))
}

func TestInvalidNumberOfDimensions(t *testing.T) {
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(observedZapCore)

	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("valid")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(123)
	for i := 0; i < 10; i++ {
		dp.Attributes().PutStr(fmt.Sprint("dim_key_", i), fmt.Sprint("dim_val_", i))
	}
	c, err := NewMetricsConverter(logger, nil, nil, nil, "_-.", false, true)
	require.NoError(t, err)
	assert.Len(t, c.MetricsToSignalFxV2(md), 1)
	// No log message should be printed
	require.Equal(t, 0, observedLogs.Len())

	mdInvalid := pmetric.NewMetrics()
	mInvalid := mdInvalid.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("invalid")
	dpInvalid := mInvalid.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetIntValue(123)

	// SFX datapoint is used for log validation
	gaugeType := sfxpb.MetricType_GAUGE
	dpSFX := &sfxpb.DataPoint{
		MetricType: &gaugeType,
		Dimensions: make([]*sfxpb.Dimension, 0, 37),
	}
	for i := 0; i < 37; i++ {
		dpInvalid.Attributes().PutStr(fmt.Sprint("dim_key_", i), fmt.Sprint("dim_val_", i))
		dpSFX.Dimensions = append(dpSFX.Dimensions, &sfxpb.Dimension{
			Key:   fmt.Sprint("dim_key_", i),
			Value: fmt.Sprint("dim_val_", i),
		})
	}
	assert.Empty(t, c.MetricsToSignalFxV2(mdInvalid))
	require.Equal(t, 1, observedLogs.Len())
	assert.Equal(t, "dropping datapoint", observedLogs.All()[0].Message)
	assert.ElementsMatch(t, []zap.Field{
		{Type: zapcore.StringType, Key: "reason", String: invalidNumberOfDimensions},
		{Type: zapcore.StringerType, Key: "datapoint", Interface: dpSFX},
		{Type: zapcore.Int64Type, Key: "number_of_dimensions", Integer: 37},
	}, observedLogs.All()[0].Context)
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
	dims map[string]any,
) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     metric,
		Timestamp:  tsMSecs,
		Value:      sfxpb.Datum{DoubleValue: &doubleVal},
		MetricType: metricType,
		Dimensions: sfxDimensions(dims),
	}
}

func int64SFxDataPoint(
	metric string,
	metricType *sfxpb.MetricType,
	dims map[string]any,
) *sfxpb.DataPoint {
	return &sfxpb.DataPoint{
		Metric:     metric,
		Timestamp:  tsMSecs,
		Value:      sfxpb.Datum{IntValue: &int64Val},
		MetricType: metricType,
		Dimensions: sfxDimensions(dims),
	}
}

func histoValue(dps *sfxpb.DataPoint, val int64) *sfxpb.DataPoint {
	dps.Value = sfxpb.Datum{IntValue: &val}
	return dps
}

func sfxDimensions(m map[string]any) []*sfxpb.Dimension {
	sfxDims := make([]*sfxpb.Dimension, 0, len(m))
	for k, v := range m {
		sfxDims = append(sfxDims, &sfxpb.Dimension{
			Key:   k,
			Value: v.(string),
		})
	}

	return sfxDims
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
			got, err := NewMetricsConverter(zap.NewNop(), nil, tt.excludes, nil, "", false, true)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
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
					}, 0, make(chan struct{}))
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
			c, err := NewMetricsConverter(zap.NewNop(), tt.fields.metricTranslator, nil, nil, tt.fields.nonAlphanumericDimChars, false, true)
			require.NoError(t, err)
			got := c.ConvertDimension(tt.args.dim)
			assert.Equal(t, tt.want, got, "ConvertDimension() = %v, want %v", got, tt.want)
		})
	}
}
