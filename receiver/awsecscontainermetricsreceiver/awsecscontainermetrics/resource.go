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
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
)

func containerResource(cm ContainerMetadata) *resourcepb.Resource {
	labels := map[string]string{}
	for k, v := range cm.Labels {
		labels[k] = v
	}
	labels["container.name"] = cm.ContainerName
	labels["container.dockerId"] = cm.DockerId
	labels["container.dockerImage"] = cm.DockerName
	return &resourcepb.Resource{
		Type:   "ecs.container",
		Labels: labels,
	}
}

func taskResource(tm TaskMetadata) *resourcepb.Resource {
	labels := map[string]string{}

	labels["task.Cluster"] = tm.Cluster
	labels["task.TaskFamily"] = tm.Family
	labels["task.TaskDefRevision"] = tm.Revision
	return &resourcepb.Resource{
		Type:   "ecs.task",
		Labels: labels,
	}
}
