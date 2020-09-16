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

package awsecscontainermetrics

import metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

func intGauge(metricName string, unit string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Unit:      unit,
			Type:      metricspb.MetricDescriptor_GAUGE_INT64,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues: labelValues,
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_Int64Value{
					Int64Value: int64(*value),
				},
			}},
		}},
	}
}

func doubleGauge(metricName string, unit string, value *float64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Unit:      unit,
			Type:      metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues: labelValues,
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_DoubleValue{
					DoubleValue: *value,
				},
			}},
		}},
	}
}

func intCumulative(metricName string, unit string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Unit:      unit,
			Type:      metricspb.MetricDescriptor_CUMULATIVE_INT64,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues: labelValues,
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_Int64Value{
					Int64Value: int64(*value),
				},
			}},
		}},
	}
}
