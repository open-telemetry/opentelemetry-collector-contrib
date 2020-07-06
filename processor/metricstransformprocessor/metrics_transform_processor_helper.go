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

import metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

// update updates the original metric content in the metric pointer.
func (mtp *metricsTransformProcessor) update(metric *metricspb.Metric, transform Transform) {
	// metric name update
	if transform.NewName != "" {
		metric.MetricDescriptor.Name = transform.NewName
	}

	for _, op := range transform.Operations {
		// update label
		if op.Action == UpdateLabel {
			mtp.updateLabelOp(metric, op)
		}
	}
}

func (mtp *metricsTransformProcessor) updateLabelOp(metric *metricspb.Metric, op Operation) {
	for idx, label := range metric.MetricDescriptor.LabelKeys {
		if label.Key != op.Label {
			continue
		}
		// label key update
		if op.NewLabel != "" {
			label.Key = op.NewLabel
		}
		// label value update
		labelValuesMapping := mtp.createLabelValueMapping(op.ValueActions)
		for _, timeseries := range metric.Timeseries {
			newValue, ok := labelValuesMapping[timeseries.LabelValues[idx].Value]
			if ok {
				timeseries.LabelValues[idx].Value = newValue
			}
		}
	}
}

// createLabelValueMapping creates a label value mapping from old value to new value.
func (mtp *metricsTransformProcessor) createLabelValueMapping(valueActions []ValueAction) map[string]string {
	mapping := make(map[string]string)
	for _, valueAction := range valueActions {
		mapping[valueAction.Value] = valueAction.NewValue
	}
	return mapping
}
