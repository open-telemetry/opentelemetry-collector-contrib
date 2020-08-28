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

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func containerLabelKeysAndValues(cm ContainerMetadata) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	var labelKeys []*metricspb.LabelKey
	var labelValues []*metricspb.LabelValue

	labelKeys = make([]*metricspb.LabelKey, 0, len(cm.Labels)+4)
	labelValues = make([]*metricspb.LabelValue, 0, len(cm.Labels)+4)

	for key, value := range cm.Labels {
		labelKeys = append(labelKeys, &metricspb.LabelKey{Key: key})
		labelValues = append(labelValues, &metricspb.LabelValue{Value: value})
	}

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "container.name"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.ContainerName})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "container.dockerId"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.DockerId})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "container.dockerName"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.DockerName})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "container.Image"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.Image})

	return labelKeys, labelValues
}

func taskLabelKeysAndValues(tm TaskMetadata) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	var labelKeys []*metricspb.LabelKey
	var labelValues []*metricspb.LabelValue

	labelKeys = make([]*metricspb.LabelKey, 0, 4)
	labelValues = make([]*metricspb.LabelValue, 0, 4)

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "task.Cluster"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Cluster})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "task.TaskFamily"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Family})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "task.TaskDefRevision"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Revision})

	return labelKeys, labelValues
}
