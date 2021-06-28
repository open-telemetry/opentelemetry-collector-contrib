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

	errSFxNoDatumValue = errors.New("no datum value present for data-point")
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
	metrics.Resize(len(sfxDataPoints))

	i := 0
	for _, sfxDataPoint := range sfxDataPoints {
		if sfxDataPoint == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}

		m := metrics.At(i)
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
		case pdata.MetricDataTypeIntGauge:
			err = fillIntDataPoint(sfxDataPoint, m.IntGauge().DataPoints())
		case pdata.MetricDataTypeIntSum:
			err = fillIntDataPoint(sfxDataPoint, m.IntSum().DataPoints())
		case pdata.MetricDataTypeDoubleGauge:
			err = fillDoubleDataPoint(sfxDataPoint, m.DoubleGauge().DataPoints())
		case pdata.MetricDataTypeDoubleSum:
			err = fillDoubleDataPoint(sfxDataPoint, m.DoubleSum().DataPoints())
		}

		if err != nil {
			numDroppedDataPoints++
			logger.Debug("SignalFx data-point datum conversion error",
				zap.Error(err),
				zap.String("metric", sfxDataPoint.GetMetric()))
			continue
		}

		i++
	}

	metrics.Resize(i)

	return md, numDroppedDataPoints
}

func fillInType(
	sfxDataPoint *sfxpb.DataPoint,
	m pdata.Metric,
) (err error) {
	// Combine metric type with the actual data point type
	sfxMetricType := sfxDataPoint.GetMetricType()
	sfxDatum := sfxDataPoint.Value
	if sfxDatum.IntValue == nil && sfxDatum.DoubleValue == nil {
		return errSFxNilDatum
	}

	switch sfxMetricType {
	case sfxpb.MetricType_GAUGE:
		switch {
		case sfxDatum.DoubleValue != nil:
			// Numerical: Periodic, instantaneous measurement of some state.
			m.SetDataType(pdata.MetricDataTypeDoubleGauge)
		case sfxDatum.IntValue != nil:
			m.SetDataType(pdata.MetricDataTypeIntGauge)
		default:
			err = fmt.Errorf("non-numeric datapoint encountered")
		}

	case sfxpb.MetricType_COUNTER:
		switch {
		case sfxDatum.DoubleValue != nil:
			m.SetDataType(pdata.MetricDataTypeDoubleSum)
			m.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
			m.DoubleSum().SetIsMonotonic(true)
		case sfxDatum.IntValue != nil:
			m.SetDataType(pdata.MetricDataTypeIntSum)
			m.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityDelta)
			m.IntSum().SetIsMonotonic(true)
		default:
			err = fmt.Errorf("non-numeric datapoint encountered")
		}

	case sfxpb.MetricType_CUMULATIVE_COUNTER:
		switch {
		case sfxDatum.DoubleValue != nil:
			m.SetDataType(pdata.MetricDataTypeDoubleSum)
			m.DoubleSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
			m.DoubleSum().SetIsMonotonic(true)
		case sfxDatum.IntValue != nil:
			m.SetDataType(pdata.MetricDataTypeIntSum)
			m.IntSum().SetAggregationTemporality(pdata.AggregationTemporalityCumulative)
			m.IntSum().SetIsMonotonic(true)
		default:
			err = fmt.Errorf("non-numeric datapoint encountered")
		}
	default:
		err = fmt.Errorf("unknown data-point type (%d)", sfxMetricType)
	}

	return err
}

func fillIntDataPoint(sfxDataPoint *sfxpb.DataPoint, dps pdata.IntDataPointSlice) error {
	if sfxDataPoint.Value.IntValue == nil {
		return errSFxNoDatumValue
	}

	dp := dps.AppendEmpty()
	dp.SetTimestamp(dpTimestamp(sfxDataPoint))
	dp.SetValue(*sfxDataPoint.Value.IntValue)
	fillInLabels(sfxDataPoint.Dimensions, dp.LabelsMap())

	return nil
}

func fillDoubleDataPoint(sfxDataPoint *sfxpb.DataPoint, dps pdata.DoubleDataPointSlice) error {
	if sfxDataPoint.Value.DoubleValue == nil {
		return errSFxNoDatumValue
	}

	dp := dps.AppendEmpty()
	dp.SetTimestamp(dpTimestamp(sfxDataPoint))
	dp.SetValue(*sfxDataPoint.Value.DoubleValue)
	fillInLabels(sfxDataPoint.Dimensions, dp.LabelsMap())

	return nil

}

func dpTimestamp(dp *sfxpb.DataPoint) pdata.Timestamp {
	// Convert from SignalFx millis to pdata nanos
	return pdata.Timestamp(dp.GetTimestamp() * 1e6)
}

func fillInLabels(
	dimensions []*sfxpb.Dimension,
	labels pdata.StringMap,
) {
	labels.Clear()
	labels.EnsureCapacity(len(dimensions))

	for _, dim := range dimensions {
		if dim == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		labels.Insert(dim.Key, dim.Value)
	}
}
