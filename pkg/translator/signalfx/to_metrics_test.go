// Copyright 2019, OpenTelemetry Authors
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

package signalfx

import (
	"strconv"
	"testing"
	"time"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func Test_ToMetrics(t *testing.T) {
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

	buildDefaultMetrics := func(typ pdata.MetricDataType, value interface{}) pdata.Metrics {
		out := pdata.NewMetrics()
		rm := out.ResourceMetrics().AppendEmpty()
		ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()
		m := ilm.Metrics().AppendEmpty()

		m.SetDataType(typ)
		m.SetName("single")

		var dps pdata.NumberDataPointSlice

		switch typ {
		case pdata.MetricDataTypeGauge:
			dps = m.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
			dps = m.Sum().DataPoints()
		}

		dp := dps.AppendEmpty()
		dp.Attributes().InsertString("k0", "v0")
		dp.Attributes().InsertString("k1", "v1")
		dp.Attributes().InsertString("k2", "v2")
		dp.Attributes().Sort()

		dp.SetTimestamp(pdata.NewTimestampFromTime(now.Truncate(time.Millisecond)))

		switch val := value.(type) {
		case int:
			dp.SetIntVal(int64(val))
		case float64:
			dp.SetDoubleVal(val)
		}

		return out
	}

	tests := []struct {
		name          string
		sfxDataPoints []*sfxpb.DataPoint
		wantMetrics   pdata.Metrics
		wantError     bool
	}{
		{
			name:          "int_gauge",
			sfxDataPoints: []*sfxpb.DataPoint{buildDefaulstSFxDataPt()},
			wantMetrics:   buildDefaultMetrics(pdata.MetricDataTypeGauge, 13),
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
			wantMetrics: buildDefaultMetrics(pdata.MetricDataTypeGauge, 13.13),
		},
		{
			name: "int_counter",
			sfxDataPoints: func() []*sfxpb.DataPoint {
				pt := buildDefaulstSFxDataPt()
				pt.MetricType = sfxTypePtr(sfxpb.MetricType_COUNTER)
				return []*sfxpb.DataPoint{pt}
			}(),
			wantMetrics: func() pdata.Metrics {
				m := buildDefaultMetrics(pdata.MetricDataTypeSum, 13)
				d := m.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
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
			wantMetrics: func() pdata.Metrics {
				m := buildDefaultMetrics(pdata.MetricDataTypeSum, 13.13)
				d := m.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0).Sum()
				d.SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
				d.SetIsMonotonic(true)
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
			wantMetrics: func() pdata.Metrics {
				md := buildDefaultMetrics(pdata.MetricDataTypeGauge, 13)
				md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).SetTimestamp(0)
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
			wantMetrics: func() pdata.Metrics {
				md := buildDefaultMetrics(pdata.MetricDataTypeGauge, 13)
				md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes().UpdateString("k0", "")
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
			wantMetrics: buildDefaultMetrics(pdata.MetricDataTypeGauge, 13),
		},
		{
			name:          "nil_datapoint_ignored",
			sfxDataPoints: []*sfxpb.DataPoint{nil, buildDefaulstSFxDataPt(), nil},
			wantMetrics:   buildDefaultMetrics(pdata.MetricDataTypeGauge, 13),
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
			wantMetrics: buildDefaultMetrics(pdata.MetricDataTypeGauge, 13),
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md, err := ToMetrics(tt.sfxDataPoints)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantMetrics, md)
		})
	}
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
