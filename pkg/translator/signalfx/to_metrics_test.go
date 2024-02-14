// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfx

import (
	"strconv"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestNumMetricTypes(t *testing.T) {
	// Assert that all values for the metric types are less than numMetricTypes.
	assert.Equal(t, len(sfxpb.MetricType_value), numMetricTypes)
	for _, v := range sfxpb.MetricType_value {
		assert.Less(t, v, int32(numMetricTypes))
	}
}

func TestToMetrics(t *testing.T) {
	now := time.Now()

	buildDefaulstSFxDataPt := func() *sfxpb.DataPoint {
		return &sfxpb.DataPoint{
			Metric:    "single",
			Timestamp: now.UnixNano() / 1e6,
			Value: sfxpb.Datum{
				IntValue: int64Ptr(13),
			},
			MetricType: sfxTypePtr(sfxpb.MetricType_GAUGE),
			Dimensions: buildNDimensions(3),
		}
	}

	tests := []struct {
		name          string
		sfxDataPoints []*sfxpb.DataPoint
		wantMetrics   pmetric.Metrics
		wantError     bool
	}{
		{
			name:          "int_gauge",
			sfxDataPoints: []*sfxpb.DataPoint{buildDefaulstSFxDataPt()},
			wantMetrics:   buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now),
		},
		{
			name: "double_gauge",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_GAUGE)
				pt.Value = sfxpb.Datum{
					DoubleValue: float64Ptr(13.13),
				}
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13.13, now),
		},
		{
			name:          "same_name_multiple_gauges",
			sfxDataPoints: []*sfxpb.DataPoint{buildDefaulstSFxDataPt(), buildDefaulstSFxDataPt()},
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now)
				dps := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
				dps.At(0).CopyTo(dps.AppendEmpty())
				return m
			}(),
		},
		{
			name: "int_counter",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_COUNTER)
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeSum, 13, now)
				d := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				d.SetIsMonotonic(true)
				return m
			}(),
		},
		{
			name: "double_counter",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_COUNTER)
				pt.Value = sfxpb.Datum{
					DoubleValue: float64Ptr(13.13),
				}
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeSum, 13.13, now)
				d := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				d.SetIsMonotonic(true)
				return m
			}(),
		},
		{
			name: "same_name_multiple_counters",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_COUNTER)
				pt.Value = sfxpb.Datum{
					DoubleValue: float64Ptr(13.13),
				}
				return []*sfxpb.DataPoint{pt, pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeSum, 13.13, now)
				d := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				d.SetIsMonotonic(true)
				d.DataPoints().At(0).CopyTo(d.DataPoints().AppendEmpty())
				return m
			}(),
		},
		{
			name: "int_cumulative",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_CUMULATIVE_COUNTER)
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeSum, 13, now)
				d := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				d.SetIsMonotonic(true)
				return m
			}(),
		},
		{
			name: "double_cumulative",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_CUMULATIVE_COUNTER)
				pt.Value = sfxpb.Datum{
					DoubleValue: float64Ptr(13.13),
				}
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeSum, 13.13, now)
				d := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				d.SetIsMonotonic(true)
				return m
			}(),
		},
		{
			name: "same_name_multiple_cumulative",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_CUMULATIVE_COUNTER)
				return []*sfxpb.DataPoint{pt, pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeSum, 13, now)
				d := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				d.SetIsMonotonic(true)
				d.DataPoints().At(0).CopyTo(d.DataPoints().AppendEmpty())
				return m
			}(),
		},
		{
			name: "same_name_different_types",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_COUNTER)
				return []*sfxpb.DataPoint{pt, buildDefaulstSFxDataPt()}
			}(),
			wantMetrics: func() pmetric.Metrics {
				m := buildDefaultMetrics(t, pmetric.MetricTypeSum, 13, now)
				d := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				d.SetIsMonotonic(true)
				// Append the Gauge metric as well.
				gm := buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now)
				gm.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).MoveTo(m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().AppendEmpty())
				return m
			}(),
		},
		{
			name: "nil_timestamp",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.Timestamp = 0
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				md := buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now)
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).SetTimestamp(0)
				return md
			}(),
		},
		{
			name: "empty_dimension_value",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.Dimensions[0].Value = ""
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: func() pmetric.Metrics {
				md := buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now)
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().PutStr("k0", "")
				return md
			}(),
		},
		{
			name: "nil_dimension_ignored",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				targetLen := 2*len(pt.Dimensions) + 1
				dimensions := make([]*sfxpb.Dimension, targetLen)
				copy(dimensions[1:], pt.Dimensions)
				assert.Equal(t, targetLen, len(dimensions))
				assert.Nil(t, dimensions[0])
				pt.Dimensions = dimensions
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now),
		},
		{
			name:          "nil_datapoint_ignored",
			sfxDataPoints: []*sfxpb.DataPoint{nil, buildDefaulstSFxDataPt(), nil},
			wantMetrics:   buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now),
		},
		{
			name: "drop_inconsistent_datapoints",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				// nil Datum
				pt0 := buildDefaulstSFxDataPt()
				pt0.Value = sfxpb.Datum{}

				// nil expected Datum value
				pt1 := buildDefaulstSFxDataPt()
				pt1.Value.IntValue = nil

				// Non-supported type
				pt2 := buildDefaulstSFxDataPt()
				pt2.MetricType = sfxTypePtr(sfxpb.MetricType_ENUM)

				// Unknown type
				pt3 := buildDefaulstSFxDataPt()
				pt3.MetricType = sfxTypePtr(sfxpb.MetricType_CUMULATIVE_COUNTER + 1)

				return []*sfxpb.DataPoint{pt0, buildDefaulstSFxDataPt(), pt1, pt2, pt3}
			}(),
			wantMetrics: buildDefaultMetrics(t, pmetric.MetricTypeGauge, 13, now),
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			to := &ToTranslator{}
			md, err := to.ToMetrics(tt.sfxDataPoints)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, pmetrictest.CompareMetrics(tt.wantMetrics, md))
		})
	}
}

func buildDefaultMetrics(t *testing.T, typ pmetric.MetricType, value any, now time.Time) pmetric.Metrics {
	out := pmetric.NewMetrics()
	rm := out.ResourceMetrics().AppendEmpty()
	ilm := rm.ScopeMetrics().AppendEmpty()
	m := ilm.Metrics().AppendEmpty()

	m.SetName("single")

	var dps pmetric.NumberDataPointSlice

	switch typ {
	case pmetric.MetricTypeGauge:
		dps = m.SetEmptyGauge().DataPoints()
	case pmetric.MetricTypeSum:
		m.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dps = m.Sum().DataPoints()
	case pmetric.MetricTypeSummary:
		require.Fail(t, "unsupported")
	case pmetric.MetricTypeHistogram:
		require.Fail(t, "unsupported")
	case pmetric.MetricTypeExponentialHistogram:
		require.Fail(t, "unsupported")
	case pmetric.MetricTypeEmpty:
		require.Fail(t, "unsupported")
	}

	dp := dps.AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.Attributes().PutStr("k2", "v2")

	dp.SetTimestamp(pcommon.NewTimestampFromTime(now.Truncate(time.Millisecond)))

	switch val := value.(type) {
	case int:
		dp.SetIntValue(int64(val))
	case float64:
		dp.SetDoubleValue(val)
	}

	return out
}

func int64Ptr(i int64) *int64 {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

func sfxTypePtr(t sfxpb.MetricType) *sfxpb.MetricType {
	return &t
}

func buildNDimensions(n uint) []*sfxpb.Dimension {
	d := make([]*sfxpb.Dimension, 0, n)
	for i := uint(0); i < n; i++ {
		idx := int(i)
		suffix := strconv.Itoa(idx)
		d = append(d, &sfxpb.Dimension{
			Key:   "k" + suffix,
			Value: "v" + suffix,
		})
	}
	return d
}
