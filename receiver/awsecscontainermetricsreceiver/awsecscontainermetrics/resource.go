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
	attributes := map[string]pdata.AttributeValue{}

	attributes[conventions.AttributeContainerName] = pdata.NewAttributeValueString(cm.ContainerName)
	attributes[conventions.AttributeContainerID] = pdata.NewAttributeValueString(cm.DockerID)
	attributes[AttributeECSDockerName] = pdata.NewAttributeValueString(cm.DockerName)

	return createResourceWithAttributes(attributes)
}

func taskResource(tm TaskMetadata) pdata.Resource {
	attributes := map[string]pdata.AttributeValue{}

	attributes[AttributeECSCluster] = pdata.NewAttributeValueString(tm.Cluster)
	attributes[AttributeECSTaskARN] = pdata.NewAttributeValueString(tm.TaskARN)
	attributes[AttributeECSTaskID] = pdata.NewAttributeValueString(getTaskIDFromARN(tm.TaskARN))
	attributes[AttributeECSTaskFamily] = pdata.NewAttributeValueString(tm.Family)
	attributes[AttributeECSTaskRevesion] = pdata.NewAttributeValueString(tm.Revision)
	attributes[AttributeECSServiceName] = pdata.NewAttributeValueString("undefined")

	return createResourceWithAttributes(attributes)
}

func createResourceWithAttributes(attributes map[string]pdata.AttributeValue) pdata.Resource {
	resource := pdata.NewResource()
	resource.InitEmpty()
	resource.Attributes().InitFromMap(attributes)
	return resource
}

func getTaskIDFromARN(arn string) string {
	if arn == "" || !strings.HasPrefix(arn, "arn:aws") {
		return ""
	}
	splits := strings.Split(arn, "/")

	return splits[len(splits)-1]
}
