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

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

func containerResource(cm ContainerMetadata) pdata.Resource {
	resource := pdata.NewResource()
	resource.Attributes().UpsertString(conventions.AttributeContainerName, cm.ContainerName)
	resource.Attributes().UpsertString(conventions.AttributeContainerID, cm.DockerID)
	resource.Attributes().UpsertString(AttributeECSDockerName, cm.DockerName)
	resource.Attributes().UpsertString(conventions.AttributeContainerImage, cm.Image)
	resource.Attributes().UpsertString(AttributeContainerImageID, cm.ImageID)
	resource.Attributes().UpsertString(conventions.AttributeContainerTag, getVersionFromIamge(cm.Image))
	resource.Attributes().UpsertString(AttributeContainerCreatedAt, cm.CreatedAt)
	resource.Attributes().UpsertString(AttributeContainerStartedAt, cm.StartedAt)
	if cm.FinishedAt != "" {
		resource.Attributes().UpsertString(AttributeContainerFinishedAt, cm.FinishedAt)
	}
	resource.Attributes().UpsertString(AttributeContainerKnownStatus, cm.KnownStatus)
	if cm.ExitCode != nil {
		resource.Attributes().UpsertInt(AttributeContainerExitCode, *cm.ExitCode)
	}
	return resource
}

func taskResource(tm TaskMetadata) pdata.Resource {
	resource := pdata.NewResource()
	region, accountID, taskID := getResourceFromARN(tm.TaskARN)
	resource.Attributes().UpsertString(AttributeECSCluster, getNameFromCluster(tm.Cluster))
	resource.Attributes().UpsertString(AttributeECSTaskARN, tm.TaskARN)
	resource.Attributes().UpsertString(AttributeECSTaskID, taskID)
	resource.Attributes().UpsertString(AttributeECSTaskFamily, tm.Family)
	resource.Attributes().UpsertString(AttributeECSTaskRevision, tm.Revision)
	resource.Attributes().UpsertString(AttributeECSServiceName, "undefined")

	resource.Attributes().UpsertString(conventions.AttributeCloudAvailabilityZone, tm.AvailabilityZone)
	resource.Attributes().UpsertString(AttributeECSTaskPullStartedAt, tm.PullStartedAt)
	resource.Attributes().UpsertString(AttributeECSTaskPullStoppedAt, tm.PullStoppedAt)
	resource.Attributes().UpsertString(AttributeECSTaskKnownStatus, tm.KnownStatus)
	resource.Attributes().UpsertString(AttributeECSTaskLaunchType, tm.LaunchType)
	resource.Attributes().UpsertString(conventions.AttributeCloudRegion, region)
	resource.Attributes().UpsertString(conventions.AttributeCloudAccount, accountID)

	return resource
}

// https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html
// The new taskARN format: New: arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id
//  Old(current): arn:aws:ecs:region:aws_account_id:task/task-id
func getResourceFromARN(arn string) (string, string, string) {
	if !strings.HasPrefix(arn, "arn:aws:ecs") {
		return "", "", ""
	}
	splits := strings.Split(arn, "/")
	taskID := splits[len(splits)-1]

	subSplits := strings.Split(splits[0], ":")
	region := subSplits[3]
	accountID := subSplits[4]

	return region, accountID, taskID
}

func getVersionFromIamge(image string) string {
	if image == "" {
		return ""
	}
	splits := strings.Split(image, ":")
	if len(splits) == 1 {
		return "latest"
	}
	return splits[len(splits)-1]
}

//The Amazon Resource Name (ARN) that identifies the cluster. The ARN contains the arn:aws:ecs namespace,
//followed by the Region of the cluster, the AWS account ID of the cluster owner, the cluster namespace,
//and then the cluster name. For example, arn:aws:ecs:region:012345678910:cluster/test.
func getNameFromCluster(cluster string) string {
	if cluster == "" || !strings.HasPrefix(cluster, "arn:aws") {
		return cluster
	}
	splits := strings.Split(cluster, "/")

	return splits[len(splits)-1]
}
