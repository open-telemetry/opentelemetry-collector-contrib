// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricstransformprocessor

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func (mtp *metricsTransformProcessor) convertResourceAttributesToLabels(metric *metricspb.Metric, resourceAttributes map[string]string, op internalOperation) {
	expectedLabelMap := make(map[string]string)

	if len(op.configOperation.ResourceAttributes) > 0 {
		for _, attribute := range op.configOperation.ResourceAttributes {
			attributeValue, ok := resourceAttributes[attribute]
			if ok {
				expectedLabelMap[attribute] = attributeValue
			}
		}
	} else {
		expectedLabelMap = resourceAttributes
	}

	for key, value := range expectedLabelMap {
		lablelKey := &metricspb.LabelKey{
			Key: key,
		}
		labelValue := &metricspb.LabelValue{
			Value:    value,
			HasValue: true,
		}

		metric.MetricDescriptor.LabelKeys = append(metric.MetricDescriptor.LabelKeys, lablelKey)
		for _, ts := range metric.Timeseries {
			ts.LabelValues = append(ts.LabelValues, labelValue)
		}
	}
}
