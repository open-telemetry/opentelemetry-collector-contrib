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
	"strings"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/translator/conventions"
)

func containerLabelKeysAndValues(cm ContainerMetadata) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	labelKeys := make([]*metricspb.LabelKey, 0, ContainerMetricsLabelLen)
	labelValues := make([]*metricspb.LabelValue, 0, ContainerMetricsLabelLen)

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: conventions.AttributeContainerName})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.ContainerName, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: conventions.AttributeContainerID})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.DockerID, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: AttributeECSDockerName})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.DockerName, HasValue: true})

	return labelKeys, labelValues
}

func taskLabelKeysAndValues(tm TaskMetadata) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	labelKeys := make([]*metricspb.LabelKey, 0, TaskMetricsLabelLen)
	labelValues := make([]*metricspb.LabelValue, 0, TaskMetricsLabelLen)

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: AttributeECSCluster})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Cluster, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: AttributeECSTaskARN})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.TaskARN, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: AttributeECSTaskID})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: getTaskIDFromARN(tm.TaskARN), HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: AttributeECSTaskFamily})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Family, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: AttributeECSTaskRevesion})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Revision, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: AttributeECSServiceName})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: "undefined", HasValue: true})

	return labelKeys, labelValues
}

func getTaskIDFromARN(arn string) string {
	if arn == "" || !strings.HasPrefix(arn, "arn:aws") {
		return ""
	}
	splits := strings.Split(arn, "/")

	return splits[len(splits)-1]
}
