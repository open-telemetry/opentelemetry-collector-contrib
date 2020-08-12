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

// updateLabelOp updates labels and label values in metric based on given operation
func (mtp *metricsTransformProcessor) updateLabelOp(metric *metricspb.Metric, mtpOp internalOperation) {
	op := mtpOp.configOperation
	for idx, label := range metric.MetricDescriptor.LabelKeys {
		if label.Key != op.Label {
			continue
		}

		if op.NewLabel != "" {
			label.Key = op.NewLabel
		}

		labelValuesMapping := mtpOp.valueActionsMapping
		for _, timeseries := range metric.Timeseries {
			newValue, ok := labelValuesMapping[timeseries.LabelValues[idx].Value]
			if ok {
				timeseries.LabelValues[idx].Value = newValue
			}
		}
	}
}
