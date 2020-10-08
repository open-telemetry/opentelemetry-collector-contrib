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

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/translator/conventions"
)

func containerResource(cm ContainerMetadata) *resourcepb.Resource {
	labels := map[string]string{}

	labels[conventions.AttributeContainerName] = cm.ContainerName
	labels[conventions.AttributeContainerID] = cm.DockerID
	labels[AttributeECSDockerName] = cm.DockerName
	return &resourcepb.Resource{
		Type:   MetricResourceType,
		Labels: labels,
	}
}

func taskResource(tm TaskMetadata) *resourcepb.Resource {
	labels := map[string]string{}

	labels[AttributeECSCluster] = tm.Cluster
	labels[AttributeECSTaskARN] = tm.TaskARN
	labels[AttributeECSTaskID] = getTaskIDFromARN(tm.TaskARN)
	labels[AttributeECSTaskFamily] = tm.Family
	labels[AttributeECSTaskRevesion] = tm.Revision
	labels[AttributeECSServiceName] = "undefined"
	return &resourcepb.Resource{
		Type:   MetricResourceType,
		Labels: labels,
	}
}

func getTaskIDFromARN(arn string) string {
	if arn == "" || !strings.HasPrefix(arn, "arn:aws") {
		return ""
	}
	splits := strings.Split(arn, "/")

	return splits[len(splits)-1]
}
