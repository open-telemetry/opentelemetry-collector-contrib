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

package signalfx // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx"

import (
	"fmt"

	"github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/multierr"
)

// ToMetrics converts SignalFx proto data points to pdata.Metrics.
func ToMetrics(sfxDataPoints []*model.DataPoint) (pdata.Metrics, error) {
	// TODO: not optimized at all, basically regenerating everything for each
	// 	data point.
	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()

	ms := ilm.Metrics()
	ms.EnsureCapacity(len(sfxDataPoints))

	var err error
	for _, sfxDataPoint := range sfxDataPoints {
		if sfxDataPoint == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		err = multierr.Append(err, setDataTypeAndPoints(sfxDataPoint, ms))
	}

	return md, err
}

func setDataTypeAndPoints(sfxDataPoint *model.DataPoint, ms pdata.MetricSlice) error {
	// Combine metric type with the actual data point type
	sfxMetricType := sfxDataPoint.GetMetricType()
	sfxDatum := sfxDataPoint.Value
	if sfxDatum.IntValue == nil && sfxDatum.DoubleValue == nil {
		return fmt.Errorf("nil datum value for data-point in metric %q", sfxDataPoint.GetMetric())
	}

	var m pdata.Metric
	switch sfxMetricType {
	case model.MetricType_GAUGE:
		m = ms.AppendEmpty()
		// Numerical: Periodic, instantaneous measurement of some state.
		m.SetDataType(pdata.MetricDataTypeGauge)
		fillNumberDataPoint(sfxDataPoint, m.Gauge().DataPoints())

	case model.MetricType_COUNTER:
		m = ms.AppendEmpty()
		m.SetDataType(pdata.MetricDataTypeSum)
		m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
		m.Sum().SetIsMonotonic(true)
		fillNumberDataPoint(sfxDataPoint, m.Sum().DataPoints())

	case model.MetricType_CUMULATIVE_COUNTER:
		m = ms.AppendEmpty()
		m.SetDataType(pdata.MetricDataTypeSum)
		m.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)
		m.Sum().SetIsMonotonic(true)
		fillNumberDataPoint(sfxDataPoint, m.Sum().DataPoints())

	default:
		return fmt.Errorf("unknown data-point type (%d) in metric %q", sfxMetricType, sfxDataPoint.GetMetric())
	}
	m.SetName(sfxDataPoint.Metric)
	return nil
}

func fillNumberDataPoint(sfxDataPoint *model.DataPoint, dps pdata.NumberDataPointSlice) {
	dp := dps.AppendEmpty()
	dp.SetTimestamp(toTimestamp(sfxDataPoint.GetTimestamp()))
	switch {
	case sfxDataPoint.Value.IntValue != nil:
		dp.SetIntVal(*sfxDataPoint.Value.IntValue)
	case sfxDataPoint.Value.DoubleValue != nil:
		dp.SetDoubleVal(*sfxDataPoint.Value.DoubleValue)
	}
	fillInAttributes(sfxDataPoint.Dimensions, dp.Attributes())
}

func fillInAttributes(
	dimensions []*model.Dimension,
	attributes pdata.Map,
) {
	attributes.Clear()
	attributes.EnsureCapacity(len(dimensions))

	for _, dim := range dimensions {
		if dim == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		attributes.InsertString(dim.Key, dim.Value)
	}
}
