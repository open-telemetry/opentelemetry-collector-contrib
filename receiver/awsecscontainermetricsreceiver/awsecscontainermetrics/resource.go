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

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

func containerResource(cm ContainerMetadata) pdata.Resource {
	resource := pdata.NewResource()
	resource.Attributes().UpsertString(conventions.AttributeContainerName, cm.ContainerName)
	resource.Attributes().UpsertString(conventions.AttributeContainerID, cm.DockerID)
	resource.Attributes().UpsertString(AttributeECSDockerName, cm.DockerName)

	return resource
}

func taskResource(tm TaskMetadata) pdata.Resource {
	resource := pdata.NewResource()
	resource.Attributes().UpsertString(AttributeECSCluster, getResourceFromARN(tm.Cluster))
	resource.Attributes().UpsertString(AttributeECSTaskARN, tm.TaskARN)
	resource.Attributes().UpsertString(AttributeECSTaskID, getResourceFromARN(tm.TaskARN))
	resource.Attributes().UpsertString(AttributeECSTaskFamily, tm.Family)
	resource.Attributes().UpsertString(AttributeECSTaskRevesion, tm.Revision)
	resource.Attributes().UpsertString(AttributeECSServiceName, "undefined")

	return resource
}

func getResourceFromARN(arn string) string {
	if arn == "" || !strings.HasPrefix(arn, "arn:aws") {
		return arn
	}
	splits := strings.Split(arn, "/")

	return splits[len(splits)-1]
}
