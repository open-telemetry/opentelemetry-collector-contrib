// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.21.0"
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

	resource.Attributes().PutStr(string(conventions.ContainerNameKey), cm.ContainerName)
	resource.Attributes().PutStr(string(conventions.ContainerIDKey), cm.DockerID)
	resource.Attributes().PutStr(attributeECSDockerName, cm.DockerName)
	resource.Attributes().PutStr(string(conventions.ContainerImageNameKey), image.Repository)
	resource.Attributes().PutStr(attributeContainerImageID, cm.ImageID)
	resource.Attributes().PutStr(string(conventions.ContainerImageTagKey), image.Tag)
	resource.Attributes().PutStr(attributeContainerCreatedAt, cm.CreatedAt)
	resource.Attributes().PutStr(attributeContainerStartedAt, cm.StartedAt)
	if cm.FinishedAt != "" {
		resource.Attributes().PutStr(attributeContainerFinishedAt, cm.FinishedAt)
	}
	resource.Attributes().PutStr(attributeContainerKnownStatus, cm.KnownStatus)
	if cm.ExitCode != nil {
		resource.Attributes().PutInt(attributeContainerExitCode, *cm.ExitCode)
	}
	return resource
}

func taskResource(tm ecsutil.TaskMetadata) pcommon.Resource {
	resource := pcommon.NewResource()
	region, accountID, taskID := getResourceFromARN(tm.TaskARN)
	resource.Attributes().PutStr(attributeECSCluster, getNameFromCluster(tm.Cluster))
	resource.Attributes().PutStr(string(conventions.AWSECSTaskARNKey), tm.TaskARN)
	resource.Attributes().PutStr(attributeECSTaskID, taskID)
	resource.Attributes().PutStr(string(conventions.AWSECSTaskFamilyKey), tm.Family)

	// Task revision: aws.ecs.task.version and aws.ecs.task.revision
	resource.Attributes().PutStr(attributeECSTaskRevision, tm.Revision)
	resource.Attributes().PutStr(string(conventions.AWSECSTaskRevisionKey), tm.Revision)

	resource.Attributes().PutStr(attributeECSServiceName, tm.ServiceName)

	resource.Attributes().PutStr(string(conventions.CloudAvailabilityZoneKey), tm.AvailabilityZone)
	resource.Attributes().PutStr(attributeECSTaskPullStartedAt, tm.PullStartedAt)
	resource.Attributes().PutStr(attributeECSTaskPullStoppedAt, tm.PullStoppedAt)
	resource.Attributes().PutStr(attributeECSTaskKnownStatus, tm.KnownStatus)

	// Task launchtype: aws.ecs.task.launch_type (raw string) and aws.ecs.launchtype (lowercase)
	resource.Attributes().PutStr(attributeECSTaskLaunchType, tm.LaunchType)
	switch lt := strings.ToLower(tm.LaunchType); lt {
	case "ec2":
		resource.Attributes().PutStr(string(conventions.AWSECSLaunchtypeKey), conventions.AWSECSLaunchtypeEC2.Value.AsString())
	case "fargate":
		resource.Attributes().PutStr(string(conventions.AWSECSLaunchtypeKey), conventions.AWSECSLaunchtypeFargate.Value.AsString())
	}

	resource.Attributes().PutStr(string(conventions.CloudRegionKey), region)
	resource.Attributes().PutStr(string(conventions.CloudAccountIDKey), accountID)

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
