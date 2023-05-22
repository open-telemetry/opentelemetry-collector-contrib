// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"

import v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

func GetInt64TimeSeries(val int64) *v1.TimeSeries {
	return GetInt64TimeSeriesWithLabels(val, nil)
}

func GetInt64TimeSeriesWithLabels(val int64, labelVals []*v1.LabelValue) *v1.TimeSeries {
	return &v1.TimeSeries{
		LabelValues: labelVals,
		Points:      []*v1.Point{{Value: &v1.Point_Int64Value{Int64Value: val}}},
	}
}

func GetDoubleTimeSeries(val float64) *v1.TimeSeries {
	return GetDoubleTimeSeriesWithLabels(val, nil)
}

func GetDoubleTimeSeriesWithLabels(val float64, labelVals []*v1.LabelValue) *v1.TimeSeries {
	return &v1.TimeSeries{
		LabelValues: labelVals,
		Points:      []*v1.Point{{Value: &v1.Point_DoubleValue{DoubleValue: val}}},
	}
}
