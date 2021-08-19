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

package signalfxreceiver

import (
	"errors"
	"fmt"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

var (
	errSFxNilDatum = errors.New("nil datum value for data-point")
)

// signalFxV2ToMetrics converts SignalFx proto data points to pdata.Metrics.
// Returning the converted data and the number of dropped data points.
func signalFxV2ToMetrics(
	logger *zap.Logger,
	sfxDataPoints []*sfxpb.DataPoint,
) (pdata.Metrics, int) {

	// TODO: not optimized at all, basically regenerating everything for each
	// 	data point.
	numDroppedDataPoints := 0
	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	ilm := rm.InstrumentationLibraryMetrics().AppendEmpty()

	metrics := ilm.Metrics()
	metrics.EnsureCapacity(len(sfxDataPoints))

	for _, sfxDataPoint := range sfxDataPoints {
		if sfxDataPoint == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}

		// fill in a new, unassociated metric as we may drop it during the process
		m := pdata.NewMetric()

		// First check if the type is convertible and the data point is consistent.
		err := fillInType(sfxDataPoint, m)
		if err != nil {
			numDroppedDataPoints++
			logger.Debug("SignalFx data-point type conversion error",
				zap.Error(err),
				zap.String("metric", sfxDataPoint.GetMetric()))
			continue
		}

		m.SetName(sfxDataPoint.Metric)

		switch m.DataType() {
		case pdata.MetricDataTypeGauge:
			fillNumberDataPoint(sfxDataPoint, m.Gauge().DataPoints())
		case pdata.MetricDataTypeSum:
			fillNumberDataPoint(sfxDataPoint, m.Sum().DataPoints())
		}

		// We know at this point we're keeping this metric
		m.CopyTo(metrics.AppendEmpty())
	}

	return md, numDroppedDataPoints
}

func fillInType(sfxDataPoint *sfxpb.DataPoint, m pdata.Metric) (err error) {
	// Combine metric type with the actual data point type
	sfxMetricType := sfxDataPoint.GetMetricType()
	sfxDatum := sfxDataPoint.Value
	if sfxDatum.IntValue == nil && sfxDatum.DoubleValue == nil {
		return errSFxNilDatum
	}

	switch sfxMetricType {
	case sfxpb.MetricType_GAUGE:
		// Numerical: Periodic, instantaneous measurement of some state.
		m.SetDataType(pdata.MetricDataTypeGauge)

	case sfxpb.MetricType_COUNTER:
		m.SetDataType(pdata.MetricDataTypeSum)
		m.Sum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
		m.Sum().SetIsMonotonic(true)

	case sfxpb.MetricType_CUMULATIVE_COUNTER:
		m.SetDataType(pdata.MetricDataTypeSum)
		m.Sum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
		m.Sum().SetIsMonotonic(true)
	default:
		err = fmt.Errorf("unknown data-point type (%d)", sfxMetricType)
	}

	return err
}

func fillNumberDataPoint(sfxDataPoint *sfxpb.DataPoint, dps pdata.NumberDataPointSlice) {
	dp := dps.AppendEmpty()
	dp.SetTimestamp(dpTimestamp(sfxDataPoint))
	switch {
	case sfxDataPoint.Value.IntValue != nil:
		dp.SetIntVal(*sfxDataPoint.Value.IntValue)
	case sfxDataPoint.Value.DoubleValue != nil:
		dp.SetDoubleVal(*sfxDataPoint.Value.DoubleValue)
	}
	fillInAttributes(sfxDataPoint.Dimensions, dp.Attributes())
}

func dpTimestamp(dp *sfxpb.DataPoint) pdata.Timestamp {
	// Convert from SignalFx millis to pdata nanos
	return pdata.Timestamp(dp.GetTimestamp() * 1e6)
}

func fillInAttributes(
	dimensions []*sfxpb.Dimension,
	attributes pdata.AttributeMap,
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
