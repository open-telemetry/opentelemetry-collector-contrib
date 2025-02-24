// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs/internal/metadata"
)

type mockMetaDataProvider struct {
	isV4           bool
	taskArnVersion int `default:"1"`
}

var _ ecsutil.MetadataProvider = (*mockMetaDataProvider)(nil)

func (md *mockMetaDataProvider) FetchTaskMetadata() (*ecsutil.TaskMetadata, error) {
	c := createTestContainer(md.isV4)
	c.DockerID = "05281997" // Simulate one "application" and one "collector" container
	cs := []ecsutil.ContainerMetadata{createTestContainer(md.isV4), c}

	var taskARN string
	switch md.taskArnVersion {
	case 1:
		taskARN = "arn:aws:ecs:us-west-2:123456789123:task/123"
	case 2:
		taskARN = "arn:aws:ecs:us-west-2:123456789123:task/my-cluster/123"
	default:
		return nil, fmt.Errorf("%s: %d", "Unsupported ECS TaskARN Spec Version", md.taskArnVersion)
	}

	tmd := &ecsutil.TaskMetadata{
		Cluster:          "my-cluster",
		TaskARN:          taskARN,
		Family:           "family",
		AvailabilityZone: "us-west-2a",
		Revision:         "26",
		Containers:       cs,
	}

	if md.isV4 {
		tmd.LaunchType = "EC2"
	}

	return tmd, nil
}

func (md *mockMetaDataProvider) FetchContainerMetadata() (*ecsutil.ContainerMetadata, error) {
	c := createTestContainer(md.isV4)
	return &c, nil
}

func Test_ecsNewDetector(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "endpoint")
	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())

	assert.NoError(t, err)
	assert.NotNil(t, d)
}

func Test_detectorReturnsIfNoEnvVars(t *testing.T) {
	d, _ := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	res, _, err := d.Detect(context.TODO())

	assert.NoError(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
}

func Test_ecsFiltersInvalidContainers(t *testing.T) {
	// Should ignore empty container
	c1 := ecsutil.ContainerMetadata{}

	// Should ignore non-normal container
	c2 := createTestContainer(true)
	c2.Type = "INTERNAL"

	// Should ignore stopped containers
	c3 := createTestContainer(true)
	c3.KnownStatus = "STOPPED"

	// Should ignore its own container
	c4 := createTestContainer(true)

	containers := []ecsutil.ContainerMetadata{c1, c2, c3, c4}

	rb := metadata.NewResourceBuilder(metadata.ResourceAttributesConfig{
		AwsEcsClusterArn:      metadata.ResourceAttributeConfig{Enabled: true},
		AwsEcsLaunchtype:      metadata.ResourceAttributeConfig{Enabled: true},
		AwsEcsTaskArn:         metadata.ResourceAttributeConfig{Enabled: true},
		AwsEcsTaskID:          metadata.ResourceAttributeConfig{Enabled: true},
		AwsEcsTaskFamily:      metadata.ResourceAttributeConfig{Enabled: true},
		AwsEcsTaskRevision:    metadata.ResourceAttributeConfig{Enabled: true},
		AwsLogGroupArns:       metadata.ResourceAttributeConfig{Enabled: true},
		AwsLogGroupNames:      metadata.ResourceAttributeConfig{Enabled: true},
		AwsLogStreamArns:      metadata.ResourceAttributeConfig{Enabled: true},
		AwsLogStreamNames:     metadata.ResourceAttributeConfig{Enabled: true},
		CloudAccountID:        metadata.ResourceAttributeConfig{Enabled: true},
		CloudAvailabilityZone: metadata.ResourceAttributeConfig{Enabled: true},
		CloudPlatform:         metadata.ResourceAttributeConfig{Enabled: true},
		CloudProvider:         metadata.ResourceAttributeConfig{Enabled: true},
		CloudRegion:           metadata.ResourceAttributeConfig{Enabled: true},
	})
	addValidLogData(containers, &c4, "123", rb)
	assert.Equal(t, 0, rb.Emit().Attributes().Len())
}

func Test_ecsDetectV4(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "endpoint")

	want := pcommon.NewResource()
	attr := want.Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_ecs")
	attr.PutStr("aws.ecs.cluster.arn", "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster")
	attr.PutStr("aws.ecs.task.arn", "arn:aws:ecs:us-west-2:123456789123:task/123")
	attr.PutStr("aws.ecs.task.id", "123")
	attr.PutStr("aws.ecs.task.family", "family")
	attr.PutStr("aws.ecs.task.revision", "26")
	attr.PutStr("cloud.region", "us-west-2")
	attr.PutStr("cloud.availability_zone", "us-west-2a")
	attr.PutStr("cloud.account.id", "123456789123")
	attr.PutStr("aws.ecs.launchtype", "ec2")
	attr.PutEmptySlice("aws.log.group.names").AppendEmpty().SetStr("group")
	attr.PutEmptySlice("aws.log.group.arns").AppendEmpty().SetStr("arn:aws:logs:us-east-1:123456789123:log-group:group")
	attr.PutEmptySlice("aws.log.stream.names").AppendEmpty().SetStr("stream")
	attr.PutEmptySlice("aws.log.stream.arns").AppendEmpty().SetStr("arn:aws:logs:us-east-1:123456789123:log-group:group:log-stream:stream")

	d := Detector{provider: &mockMetaDataProvider{isV4: true, taskArnVersion: 1}, rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	got, _, err := d.Detect(context.TODO())

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, want.Attributes().AsRaw(), got.Attributes().AsRaw())
}

func Test_ecsDetectV4WithTaskArnVersion2(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "endpoint")

	want := pcommon.NewResource()
	attr := want.Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_ecs")
	attr.PutStr("aws.ecs.cluster.arn", "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster")
	attr.PutStr("aws.ecs.task.arn", "arn:aws:ecs:us-west-2:123456789123:task/my-cluster/123")
	attr.PutStr("aws.ecs.task.id", "123")
	attr.PutStr("aws.ecs.task.family", "family")
	attr.PutStr("aws.ecs.task.revision", "26")
	attr.PutStr("cloud.region", "us-west-2")
	attr.PutStr("cloud.availability_zone", "us-west-2a")
	attr.PutStr("cloud.account.id", "123456789123")
	attr.PutStr("aws.ecs.launchtype", "ec2")
	attr.PutEmptySlice("aws.log.group.names").AppendEmpty().SetStr("group")
	attr.PutEmptySlice("aws.log.group.arns").AppendEmpty().SetStr("arn:aws:logs:us-east-1:123456789123:log-group:group")
	attr.PutEmptySlice("aws.log.stream.names").AppendEmpty().SetStr("stream")
	attr.PutEmptySlice("aws.log.stream.arns").AppendEmpty().SetStr("arn:aws:logs:us-east-1:123456789123:log-group:group:log-stream:stream")

	d := Detector{provider: &mockMetaDataProvider{isV4: true, taskArnVersion: 2}, rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	got, _, err := d.Detect(context.TODO())

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, want.Attributes().AsRaw(), got.Attributes().AsRaw())
}

func Test_ecsDetectV3(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV3EnvVar, "endpoint")

	want := pcommon.NewResource()
	attr := want.Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_ecs")
	attr.PutStr("aws.ecs.cluster.arn", "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster")
	attr.PutStr("aws.ecs.task.arn", "arn:aws:ecs:us-west-2:123456789123:task/123")
	attr.PutStr("aws.ecs.task.id", "123")
	attr.PutStr("aws.ecs.task.family", "family")
	attr.PutStr("aws.ecs.task.revision", "26")
	attr.PutStr("cloud.region", "us-west-2")
	attr.PutStr("cloud.availability_zone", "us-west-2a")
	attr.PutStr("cloud.account.id", "123456789123")

	d := Detector{provider: &mockMetaDataProvider{isV4: false, taskArnVersion: 1}, rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	got, _, err := d.Detect(context.TODO())

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, want.Attributes().AsRaw(), got.Attributes().AsRaw())
}

func Test_ecsDetectV3WithTaskArnVersion2(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV3EnvVar, "endpoint")

	want := pcommon.NewResource()
	attr := want.Attributes()
	attr.PutStr("cloud.provider", "aws")
	attr.PutStr("cloud.platform", "aws_ecs")
	attr.PutStr("aws.ecs.cluster.arn", "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster")
	attr.PutStr("aws.ecs.task.arn", "arn:aws:ecs:us-west-2:123456789123:task/my-cluster/123")
	attr.PutStr("aws.ecs.task.id", "123")
	attr.PutStr("aws.ecs.task.family", "family")
	attr.PutStr("aws.ecs.task.revision", "26")
	attr.PutStr("cloud.region", "us-west-2")
	attr.PutStr("cloud.availability_zone", "us-west-2a")
	attr.PutStr("cloud.account.id", "123456789123")

	d := Detector{provider: &mockMetaDataProvider{isV4: false, taskArnVersion: 2}, rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())}
	got, _, err := d.Detect(context.TODO())

	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, want.Attributes().AsRaw(), got.Attributes().AsRaw())
}

func createTestContainer(isV4 bool) ecsutil.ContainerMetadata {
	c := ecsutil.ContainerMetadata{
		DockerID:    "123",
		Type:        "NORMAL",
		KnownStatus: "RUNNING",
	}

	if isV4 {
		c.LogDriver = "awslogs"
		c.ContainerARN = "arn:aws:ecs"
		c.LogOptions = ecsutil.LogOptions{LogGroup: "group", Region: "us-east-1", Stream: "stream"}
	}

	return c
}
