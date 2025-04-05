// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.21.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

func TestContainerResource(t *testing.T) {
	cm := ecsutil.ContainerMetadata{
		ContainerName: "container-1",
		DockerID:      "001",
		DockerName:    "docker-container-1",
		Image:         "nginx:v1.0",
		ImageID:       "sha256:8cf1bfb43ff5d9b05af9b6b63983440f137",
		CreatedAt:     "2020-07-30T22:12:29.837074927Z",
		StartedAt:     "2020-07-30T22:12:31.153459485Z",
		KnownStatus:   "RUNNING",
	}

	r := containerResource(cm, zap.NewNop())
	require.NotNil(t, r)
	attrMap := r.Attributes()
	require.Equal(t, 9, attrMap.Len())
	expected := map[string]string{
		conventions.AttributeContainerName:      "container-1",
		conventions.AttributeContainerID:        "001",
		attributeECSDockerName:                  "docker-container-1",
		conventions.AttributeContainerImageName: "nginx",
		attributeContainerImageID:               "sha256:8cf1bfb43ff5d9b05af9b6b63983440f137",
		conventions.AttributeContainerImageTag:  "v1.0",
		attributeContainerCreatedAt:             "2020-07-30T22:12:29.837074927Z",
		attributeContainerStartedAt:             "2020-07-30T22:12:31.153459485Z",
		attributeContainerKnownStatus:           "RUNNING",
	}

	verifyAttributeMap(t, expected, attrMap)
}

func TestContainerResourceForStoppedContainer(t *testing.T) {
	var exitCode int64 = 2
	cm := ecsutil.ContainerMetadata{
		ContainerName: "container-1",
		DockerID:      "001",
		DockerName:    "docker-container-1",
		Image:         "nginx:v1.0",
		ImageID:       "sha256:8cf1bfb43ff5d9b05af9b6b63983440f137",
		CreatedAt:     "2020-07-30T22:12:29.837074927Z",
		StartedAt:     "2020-07-30T22:12:31.153459485Z",
		FinishedAt:    "2020-07-31T22:12:29.837074927Z",
		KnownStatus:   "STOPPED",
		ExitCode:      &exitCode,
	}

	r := containerResource(cm, zap.NewNop())
	require.NotNil(t, r)
	attrMap := r.Attributes()
	getExitCodeAd, found := attrMap.Get(attributeContainerExitCode)
	require.True(t, found)
	require.EqualValues(t, 2, getExitCodeAd.Int())
	require.Equal(t, 11, attrMap.Len())
	expected := map[string]string{
		conventions.AttributeContainerName:      "container-1",
		conventions.AttributeContainerID:        "001",
		attributeECSDockerName:                  "docker-container-1",
		conventions.AttributeContainerImageName: "nginx",
		attributeContainerImageID:               "sha256:8cf1bfb43ff5d9b05af9b6b63983440f137",
		conventions.AttributeContainerImageTag:  "v1.0",
		attributeContainerCreatedAt:             "2020-07-30T22:12:29.837074927Z",
		attributeContainerStartedAt:             "2020-07-30T22:12:31.153459485Z",
		attributeContainerFinishedAt:            "2020-07-31T22:12:29.837074927Z",
		attributeContainerKnownStatus:           "STOPPED",
	}

	verifyAttributeMap(t, expected, attrMap)
}

func TestTaskResource(t *testing.T) {
	tm := ecsutil.TaskMetadata{
		Cluster:          "cluster-1",
		TaskARN:          "arn:aws:ecs:us-west-2:111122223333:task/default/158d1c8083dd49d6b527399fd6414f5c",
		Family:           "task-def-family-1",
		Revision:         "v1.2",
		AvailabilityZone: "us-west-2d",
		PullStartedAt:    "2020-10-02T00:43:06.202617438Z",
		PullStoppedAt:    "2020-10-02T00:43:06.31288465Z",
		KnownStatus:      "RUNNING",
		LaunchType:       "EC2",
		ServiceName:      "MyService",
	}
	r := taskResource(tm)
	require.NotNil(t, r)

	attrMap := r.Attributes()
	require.Equal(t, 15, attrMap.Len())
	expected := map[string]string{
		attributeECSCluster:                        "cluster-1",
		conventions.AttributeAWSECSTaskARN:         "arn:aws:ecs:us-west-2:111122223333:task/default/158d1c8083dd49d6b527399fd6414f5c",
		attributeECSTaskID:                         "158d1c8083dd49d6b527399fd6414f5c",
		conventions.AttributeAWSECSTaskFamily:      "task-def-family-1",
		attributeECSTaskRevision:                   "v1.2",
		conventions.AttributeAWSECSTaskRevision:    "v1.2",
		conventions.AttributeCloudAvailabilityZone: "us-west-2d",
		attributeECSTaskPullStartedAt:              "2020-10-02T00:43:06.202617438Z",
		attributeECSTaskPullStoppedAt:              "2020-10-02T00:43:06.31288465Z",
		attributeECSTaskKnownStatus:                "RUNNING",
		attributeECSTaskLaunchType:                 "EC2",
		conventions.AttributeAWSECSLaunchtype:      conventions.AttributeAWSECSLaunchtypeEC2,
		conventions.AttributeCloudRegion:           "us-west-2",
		conventions.AttributeCloudAccountID:        "111122223333",
		attributeECSServiceName:                    "MyService",
	}

	verifyAttributeMap(t, expected, attrMap)
}

func TestTaskResourceWithClusterARN(t *testing.T) {
	tm := ecsutil.TaskMetadata{
		Cluster:          "arn:aws:ecs:us-west-2:803860917211:cluster/main-cluster",
		TaskARN:          "arn:aws:ecs:us-west-2:803860917211:cluster/main-cluster/c8083dd49d6b527399fd6414",
		Family:           "task-def-family-1",
		Revision:         "v1.2",
		AvailabilityZone: "us-west-2d",
		PullStartedAt:    "2020-10-02T00:43:06.202617438Z",
		PullStoppedAt:    "2020-10-02T00:43:06.31288465Z",
		KnownStatus:      "RUNNING",
		LaunchType:       "EC2",
		ServiceName:      "MyService",
	}
	r := taskResource(tm)
	require.NotNil(t, r)

	attrMap := r.Attributes()
	require.Equal(t, 15, attrMap.Len())

	expected := map[string]string{
		attributeECSCluster:                        "main-cluster",
		conventions.AttributeAWSECSTaskARN:         "arn:aws:ecs:us-west-2:803860917211:cluster/main-cluster/c8083dd49d6b527399fd6414",
		attributeECSTaskID:                         "c8083dd49d6b527399fd6414",
		conventions.AttributeAWSECSTaskFamily:      "task-def-family-1",
		attributeECSTaskRevision:                   "v1.2",
		conventions.AttributeAWSECSTaskRevision:    "v1.2",
		conventions.AttributeCloudAvailabilityZone: "us-west-2d",
		attributeECSTaskPullStartedAt:              "2020-10-02T00:43:06.202617438Z",
		attributeECSTaskPullStoppedAt:              "2020-10-02T00:43:06.31288465Z",
		attributeECSTaskKnownStatus:                "RUNNING",
		attributeECSTaskLaunchType:                 "EC2",
		conventions.AttributeAWSECSLaunchtype:      conventions.AttributeAWSECSLaunchtypeEC2,
		conventions.AttributeCloudRegion:           "us-west-2",
		conventions.AttributeCloudAccountID:        "803860917211",
		attributeECSServiceName:                    "MyService",
	}

	verifyAttributeMap(t, expected, attrMap)
}

func verifyAttributeMap(t *testing.T, expected map[string]string, found pcommon.Map) {
	for key, val := range expected {
		attributeVal, found := found.Get(key)
		require.True(t, found)

		require.Equal(t, val, attributeVal.Str())
	}
}

func TestGetResourceFromARN(t *testing.T) {
	region, accountID, taskID := getResourceFromARN("arn:aws:ecs:us-west-2:803860917211:task/test200/d22aaa11bf0e4ab19c2c940a1cbabbee")
	require.Equal(t, "us-west-2", region)
	require.Equal(t, "803860917211", accountID)
	require.Equal(t, "d22aaa11bf0e4ab19c2c940a1cbabbee", taskID)
	region, accountID, taskID = getResourceFromARN("")
	require.LessOrEqual(t, 0, len(region))
	require.LessOrEqual(t, 0, len(accountID))
	require.LessOrEqual(t, 0, len(taskID))
	region, accountID, taskID = getResourceFromARN("notarn:aws:ecs:us-west-2:803860917211:task/test2")
	require.LessOrEqual(t, 0, len(region))
	require.LessOrEqual(t, 0, len(accountID))
	require.LessOrEqual(t, 0, len(taskID))
}

func TestGetNameFromCluster(t *testing.T) {
	clusterName := getNameFromCluster("arn:aws:ecs:region:012345678910:cluster/test")
	require.Equal(t, "test", clusterName)

	clusterName = getNameFromCluster("not-arn:aws:something/001")
	require.Equal(t, "not-arn:aws:something/001", clusterName)

	clusterName = getNameFromCluster("")
	require.LessOrEqual(t, 0, len(clusterName))
}
