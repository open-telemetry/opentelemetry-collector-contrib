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

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/docker"
)

func containerResource(cm ecsutil.ContainerMetadata, logger *zap.Logger) pcommon.Resource {
	resource := pcommon.NewResource()

	image, err := docker.ParseImageName(cm.Image)
	if err != nil {
		docker.LogParseError(err, cm.Image, logger)
	}

	resource.Attributes().PutString(conventions.AttributeContainerName, cm.ContainerName)
	resource.Attributes().PutString(conventions.AttributeContainerID, cm.DockerID)
	resource.Attributes().PutString(attributeECSDockerName, cm.DockerName)
	resource.Attributes().PutString(conventions.AttributeContainerImageName, image.Repository)
	resource.Attributes().PutString(attributeContainerImageID, cm.ImageID)
	resource.Attributes().PutString(conventions.AttributeContainerImageTag, image.Tag)
	resource.Attributes().PutString(attributeContainerCreatedAt, cm.CreatedAt)
	resource.Attributes().PutString(attributeContainerStartedAt, cm.StartedAt)
	if cm.FinishedAt != "" {
		resource.Attributes().PutString(attributeContainerFinishedAt, cm.FinishedAt)
	}
	resource.Attributes().PutString(attributeContainerKnownStatus, cm.KnownStatus)
	if cm.ExitCode != nil {
		resource.Attributes().PutInt(attributeContainerExitCode, *cm.ExitCode)
	}
	return resource
}

func taskResource(tm ecsutil.TaskMetadata) pcommon.Resource {
	resource := pcommon.NewResource()
	region, accountID, taskID := getResourceFromARN(tm.TaskARN)
	resource.Attributes().PutString(attributeECSCluster, getNameFromCluster(tm.Cluster))
	resource.Attributes().PutString(conventions.AttributeAWSECSTaskARN, tm.TaskARN)
	resource.Attributes().PutString(attributeECSTaskID, taskID)
	resource.Attributes().PutString(conventions.AttributeAWSECSTaskFamily, tm.Family)

	// Task revision: aws.ecs.task.version and aws.ecs.task.revision
	resource.Attributes().PutString(attributeECSTaskRevision, tm.Revision)
	resource.Attributes().PutString(conventions.AttributeAWSECSTaskRevision, tm.Revision)

	resource.Attributes().PutString(attributeECSServiceName, "undefined")

	resource.Attributes().PutString(conventions.AttributeCloudAvailabilityZone, tm.AvailabilityZone)
	resource.Attributes().PutString(attributeECSTaskPullStartedAt, tm.PullStartedAt)
	resource.Attributes().PutString(attributeECSTaskPullStoppedAt, tm.PullStoppedAt)
	resource.Attributes().PutString(attributeECSTaskKnownStatus, tm.KnownStatus)

	// Task launchtype: aws.ecs.task.launch_type (raw string) and aws.ecs.launchtype (lowercase)
	resource.Attributes().PutString(attributeECSTaskLaunchType, tm.LaunchType)
	switch lt := strings.ToLower(tm.LaunchType); lt {
	case "ec2":
		resource.Attributes().PutString(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeEC2)
	case "fargate":
		resource.Attributes().PutString(conventions.AttributeAWSECSLaunchtype, conventions.AttributeAWSECSLaunchtypeFargate)
	}

	resource.Attributes().PutString(conventions.AttributeCloudRegion, region)
	resource.Attributes().PutString(conventions.AttributeCloudAccountID, accountID)

	return resource
}

// https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html
// The new taskARN format: New: arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id
//
//	Old(current): arn:aws:ecs:region:aws_account_id:task/task-id
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

// The Amazon Resource Name (ARN) that identifies the cluster. The ARN contains the arn:aws:ecs namespace,
// followed by the Region of the cluster, the AWS account ID of the cluster owner, the cluster namespace,
// and then the cluster name. For example, arn:aws:ecs:region:012345678910:cluster/test.
func getNameFromCluster(cluster string) string {
	if cluster == "" || !strings.HasPrefix(cluster, "arn:aws") {
		return cluster
	}
	splits := strings.Split(cluster, "/")

	return splits[len(splits)-1]
}
