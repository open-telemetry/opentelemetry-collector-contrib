// Copyright The OpenTelemetry Authors
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
