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
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
)

func (mtp *metricsTransformProcessor) copyResourceAttributeOp(metric *metricspb.Metric, op internalOperation, resource *resourcepb.Resource) {
	var attrVal metricspb.LabelValue

	if !attrVal.HasValue && resource != nil {
		if val, ok := resource.Labels[op.configOperation.AttributeName]; ok {
			attrVal.Value = val
			attrVal.HasValue = true
		}
	}

	if !attrVal.HasValue {
		return
	}

	metric.MetricDescriptor.LabelKeys = append(metric.MetricDescriptor.LabelKeys, &metricspb.LabelKey{
		Key: op.configOperation.AttributeName,
	})
	for _, ts := range metric.Timeseries {
		ts.LabelValues = append(ts.LabelValues, &attrVal)
	}
}
