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
)

func containerLabelKeysAndValues(cm ContainerMetadata) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	labelKeys := make([]*metricspb.LabelKey, 0, 3)
	labelValues := make([]*metricspb.LabelValue, 0, 3)

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.container-name"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.ContainerName, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.docker-id"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.DockerID, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.docker-name"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: cm.DockerName, HasValue: true})

	return labelKeys, labelValues
}

func taskLabelKeysAndValues(tm TaskMetadata) ([]*metricspb.LabelKey, []*metricspb.LabelValue) {
	labelKeys := make([]*metricspb.LabelKey, 0, 6)
	labelValues := make([]*metricspb.LabelValue, 0, 6)

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.cluster"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Cluster, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.task-arn"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.TaskARN, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.task-id"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: getTaskIDFromARN(tm.TaskARN), HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.task-definition-family"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Family, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.task-definition-version"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: tm.Revision, HasValue: true})

	labelKeys = append(labelKeys, &metricspb.LabelKey{Key: "ecs.service"})
	labelValues = append(labelValues, &metricspb.LabelValue{Value: "undefined", HasValue: true})

	return labelKeys, labelValues
}

func getTaskIDFromARN(arn string) string {
	if !strings.HasPrefix(arn, "arn:aws") || arn == "" {
		return ""
	}
	splits := strings.Split(arn, "/")

	return splits[len(splits)-1]
}
