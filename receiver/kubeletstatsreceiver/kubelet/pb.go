// Copyright 2020, OpenTelemetry Authors
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

package kubelet

import metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

func cumulativeInt(metricName string, value *uint64) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: metricName,
			Unit: "By",
			Type: metricspb.MetricDescriptor_CUMULATIVE_INT64,
		},
		Timeseries: []*metricspb.TimeSeries{{
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_Int64Value{
					Int64Value: int64(*value),
				},
			}},
		}},
	}
}

func cumulativeDouble(metricName string, value *float64) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: metricName,
			Unit: "s",
			Type: metricspb.MetricDescriptor_CUMULATIVE_DOUBLE,
		},
		Timeseries: []*metricspb.TimeSeries{{
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_DoubleValue{
					DoubleValue: *value,
				},
			}},
		}},
	}
}

func intGauge(metricName string, units string, value *uint64) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return intGaugeWithDescription(metricName, units, "", value)
}

func intGaugeWithDescription(metricName string, units string, description string, value *uint64) *metricspb.Metric {
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:        metricName,
			Unit:        units,
			Description: description,
			Type:        metricspb.MetricDescriptor_GAUGE_INT64,
		},
		Timeseries: []*metricspb.TimeSeries{{
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_Int64Value{
					Int64Value: int64(*value),
				},
			}},
		}},
	}
}

func doubleGauge(metricName string, units string, value *float64) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name: metricName,
			Unit: units,
			Type: metricspb.MetricDescriptor_GAUGE_DOUBLE,
		},
		Timeseries: []*metricspb.TimeSeries{{
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_DoubleValue{
					DoubleValue: *value,
				},
			}},
		}},
	}
}
