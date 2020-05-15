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
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.uber.org/zap"
)

var (
	errSFxNilDatum = errors.New("nil datum value for data-point")

	errSFxUnexpectedInt64DatumType   = errors.New("datum value type of int64 is unexpected")
	errSFxUnexpectedFloat64DatumType = errors.New("datum value type of float64 is unexpected")
	errSFxUnexpectedStringDatumType  = errors.New("datum value type of string is unexpected")
	errSFxStringDatumNotNumber       = errors.New("datum string cannot be parsed to a number")
	errSFxNoDatumValue               = errors.New("no datum value present for data-point")
)

// SignalFxV2ToMetricsData converts SignalFx proto data points to
// consumerdata.MetricsData. Returning the converted data and the number of
// dropped time series.
func SignalFxV2ToMetricsData(
	logger *zap.Logger,
	sfxDataPoints []*sfxpb.DataPoint,
) (*consumerdata.MetricsData, int) {

	// TODO: not optimized at all, basically regenerating everything for each
	// 	data point.
	numDroppedTimeSeries := 0
	md := &consumerdata.MetricsData{}
	metrics := make([]*metricspb.Metric, 0, len(sfxDataPoints))
	for _, sfxDataPoint := range sfxDataPoints {
		if sfxDataPoint == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}

		// First check if the type is convertible and the data point is consistent.
		metricType, err := convertType(sfxDataPoint)
		if err != nil {
			numDroppedTimeSeries++
			logger.Debug("SignalFx data-point type conversion error",
				zap.Error(err),
				zap.String("metric", sfxDataPoint.GetMetric()))
			continue
		}
		point, err := buildPoint(sfxDataPoint, metricType)
		if err != nil {
			numDroppedTimeSeries++
			logger.Debug("SignalFx data-point datum conversion error",
				zap.Error(err),
				zap.String("metric", sfxDataPoint.GetMetric()))
			continue
		}

		labelKeys, labelValues := buildLabelKeysAndValues(sfxDataPoint.Dimensions)
		descriptor := buildDescriptor(sfxDataPoint, labelKeys, metricType)
		ts := &metricspb.TimeSeries{
			// TODO: StartTimestamp can be set if each cumulative time series are
			//  	tracked but right now it is not clear if it brings benefits.
			//		Perhaps as an option so cost is "pay for play".
			LabelValues: labelValues,
			Points:      []*metricspb.Point{point},
		}
		metric := &metricspb.Metric{
			MetricDescriptor: descriptor,
			Timeseries:       []*metricspb.TimeSeries{ts},
		}
		metrics = append(metrics, metric)
	}

	md.Metrics = metrics
	return md, numDroppedTimeSeries
}

func convertType(
	sfxDataPoint *sfxpb.DataPoint,
) (descType metricspb.MetricDescriptor_Type, err error) {

	// Combine metric type with the actual data point type
	sfxMetricType := sfxDataPoint.GetMetricType()
	sfxDatum := sfxDataPoint.Value
	if sfxDatum == nil {
		return metricspb.MetricDescriptor_UNSPECIFIED, errSFxNilDatum
	}

	switch sfxMetricType {
	case sfxpb.MetricType_GAUGE:
		// Numerical: Periodic, instantaneous measurement of some state.
		descType = metricspb.MetricDescriptor_GAUGE_DOUBLE
		if sfxDatum.IntValue != nil {
			descType = metricspb.MetricDescriptor_GAUGE_INT64
		}

	case sfxpb.MetricType_COUNTER, sfxpb.MetricType_CUMULATIVE_COUNTER:
		// COUNTER:  Count of occurrences. Generally non-negative integers.
		// CUMULATIVE_COUNTER: Tracks a value that increases over time, where
		// only the difference is important.
		descType = metricspb.MetricDescriptor_CUMULATIVE_DOUBLE
		if sfxDatum.IntValue != nil {
			descType = metricspb.MetricDescriptor_CUMULATIVE_INT64
		}

	case sfxpb.MetricType_ENUM:
		// String: Used for non-continuous quantities (that is, measurements where there is a fixed
		// set of meaningful values). This is essentially a special case of gauge.
		// Attempt to treat it as a numeric gauge.
		descType = metricspb.MetricDescriptor_GAUGE_DOUBLE

	default:
		err = fmt.Errorf("unknown data-point type (%d)", sfxMetricType)
	}

	return descType, err
}

func buildPoint(
	sfxDataPoint *sfxpb.DataPoint,
	expectedMetricType metricspb.MetricDescriptor_Type,
) (*metricspb.Point, error) {
	if sfxDataPoint.Value == nil {
		return nil, errSFxNilDatum
	}

	p := &metricspb.Point{
		Timestamp: convertTimestamp(sfxDataPoint.GetTimestamp()),
	}

	switch {
	case sfxDataPoint.Value.IntValue != nil:
		mismatch := expectedMetricType != metricspb.MetricDescriptor_CUMULATIVE_INT64 &&
			expectedMetricType != metricspb.MetricDescriptor_GAUGE_INT64
		if mismatch {
			return nil, errSFxUnexpectedInt64DatumType
		}
		p.Value = &metricspb.Point_Int64Value{Int64Value: *sfxDataPoint.Value.IntValue}

	case sfxDataPoint.Value.DoubleValue != nil:
		mismatch := expectedMetricType != metricspb.MetricDescriptor_CUMULATIVE_DOUBLE &&
			expectedMetricType != metricspb.MetricDescriptor_GAUGE_DOUBLE
		if mismatch {
			return nil, errSFxUnexpectedFloat64DatumType
		}
		p.Value = &metricspb.Point_DoubleValue{DoubleValue: *sfxDataPoint.Value.DoubleValue}

	case sfxDataPoint.Value.StrValue != nil:
		if expectedMetricType != metricspb.MetricDescriptor_GAUGE_DOUBLE {
			return nil, errSFxUnexpectedStringDatumType
		}
		dbl, err := strconv.ParseFloat(*sfxDataPoint.Value.StrValue, 64)
		if err != nil {
			return nil, errSFxStringDatumNotNumber
		}
		p.Value = &metricspb.Point_DoubleValue{DoubleValue: dbl}

	default:
		return nil, errSFxNoDatumValue
	}

	return p, nil
}

func convertTimestamp(msec int64) *timestamp.Timestamp {
	if msec == 0 {
		return nil
	}

	ts := &timestamp.Timestamp{
		Seconds: msec / 1e3,
		Nanos:   int32(msec%1e3) * 1e6,
	}
	return ts
}

func buildDescriptor(
	sfxDataPoint *sfxpb.DataPoint,
	labelKeys []*metricspb.LabelKey,
	metricType metricspb.MetricDescriptor_Type,
) *metricspb.MetricDescriptor {

	// TODO: Evaluate performance impact with different datasets to see if it
	//  is worth to cache these.
	descriptor := &metricspb.MetricDescriptor{
		Name: sfxDataPoint.GetMetric(),
		// Description: no value to go here
		// Unit:        no value to go here
		Type:      metricType,
		LabelKeys: labelKeys,
	}

	return descriptor
}

func buildLabelKeysAndValues(
	dimensions []*sfxpb.Dimension,
) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	keys := make([]*metricspb.LabelKey, 0, len(dimensions))
	values := make([]*metricspb.LabelValue, 0, len(dimensions))
	for _, dim := range dimensions {
		if dim == nil {
			// TODO: Log or metric for this odd ball?
			continue
		}
		lk := &metricspb.LabelKey{Key: *dim.Key}
		keys = append(keys, lk)

		lv := &metricspb.LabelValue{}
		if dim.Value != nil {
			lv.Value = *dim.Value
			lv.HasValue = true
		}
		values = append(values, lv)
	}
	return keys, values
}
