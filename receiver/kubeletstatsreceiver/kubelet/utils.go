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

import (
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func applyCurrentTime(metrics []*metricspb.Metric, t time.Time) []*metricspb.Metric {
	currentTime := timestamppb.New(t)
	for _, metric := range metrics {
		if metric != nil {
			metric.Timeseries[0].Points[0].Timestamp = currentTime
		}
	}
	return metrics
}

// todo put this in a common lib
func labels(labels map[string]string, descriptions map[string]string) (
	[]*metricspb.LabelKey, []*metricspb.LabelValue,
) {
	var keys []*metricspb.LabelKey
	var values []*metricspb.LabelValue
	for key, val := range labels {
		labelKey := &metricspb.LabelKey{Key: key}
		desc, hasDesc := descriptions[key]
		if hasDesc {
			labelKey.Description = desc
		}
		keys = append(keys, labelKey)
		values = append(values, &metricspb.LabelValue{
			Value:    val,
			HasValue: true,
		})
	}
	return keys, values
}

func applyLabels(metric *metricspb.Metric, attrs map[string]string) {
	if metric != nil {
		metric.MetricDescriptor.LabelKeys, metric.Timeseries[0].LabelValues = labels(attrs, nil)
	}
}
