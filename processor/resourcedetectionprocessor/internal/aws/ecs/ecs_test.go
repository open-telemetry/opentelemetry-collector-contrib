// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecs

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

type mockMetaDataProvider struct {
	isV4 bool
}

var _ ecsMetadataProvider = (*mockMetaDataProvider)(nil)

func (md *mockMetaDataProvider) fetchTaskMetaData(tmde string) (*TaskMetaData, error) {
	c := createTestContainer(md.isV4)
	c.DockerID = "05281997" // Simulate one "application" and one "collector" container
	cs := []Container{createTestContainer(md.isV4), c}
	tmd := &TaskMetaData{
		Cluster:          "my-cluster",
		TaskARN:          "arn:aws:ecs:us-west-2:123456789123:task/123",
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

func (md *mockMetaDataProvider) fetchContainerMetaData(string) (*Container, error) {
	c := createTestContainer(md.isV4)
	return &c, nil
}

func Test_ecsNewDetector(t *testing.T) {
	d, err := NewDetector(componenttest.NewNopProcessorCreateSettings(), nil)

	assert.NotNil(t, d)
	assert.Nil(t, err)
}

func Test_detectorReturnsIfNoEnvVars(t *testing.T) {
	os.Clearenv()
	d, _ := NewDetector(componenttest.NewNopProcessorCreateSettings(), nil)
	res, _, err := d.Detect(context.TODO())

	assert.Nil(t, err)
	assert.Equal(t, 0, res.Attributes().Len())
}

func Test_ecsPrefersLatestTmde(t *testing.T) {
	os.Clearenv()
	os.Setenv(tmde3EnvVar, "3")
	os.Setenv(tmde4EnvVar, "4")

	tmde := getTmdeFromEnv()

	assert.Equal(t, "4", tmde)
}

func Test_ecsFiltersInvalidContainers(t *testing.T) {
	// Should ignore empty container
	c1 := Container{}

	// Should ignore non-normal container
	c2 := createTestContainer(true)
	c2.Type = "INTERNAL"

	// Should ignore stopped containers
	c3 := createTestContainer(true)
	c3.KnownStatus = "STOPPED"

	// Should ignore its own container
	c4 := createTestContainer(true)

	containers := []Container{c1, c2, c3, c4}

	ld := getValidLogData(containers, &c4, "123")

	for _, attrib := range ld {
		assert.Equal(t, 0, attrib.ArrayVal().Len())
	}
}

func Test_ecsDetectV4(t *testing.T) {
	os.Clearenv()
	os.Setenv(tmde4EnvVar, "endpoint")

	want := pdata.NewResource()
	attr := want.Attributes()
	attr.InsertString("cloud.provider", "aws")
	attr.InsertString("cloud.platform", "aws_ecs")
	attr.InsertString("aws.ecs.cluster.arn", "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster")
	attr.InsertString("aws.ecs.task.arn", "arn:aws:ecs:us-west-2:123456789123:task/123")
	attr.InsertString("aws.ecs.task.family", "family")
	attr.InsertString("aws.ecs.task.revision", "26")
	attr.InsertString("cloud.region", "us-west-2")
	attr.InsertString("cloud.availability_zone", "us-west-2a")
	attr.InsertString("cloud.account.id", "123456789123")
	attr.InsertString("aws.ecs.launchtype", "ec2")

	attribFields := []string{"aws.log.group.names", "aws.log.group.arns", "aws.log.stream.names", "aws.log.stream.arns"}
	attribVals := []string{"group", "arn:aws:logs:us-east-1:123456789123:log-group:group", "stream", "arn:aws:logs:us-east-1:123456789123:log-group:group:log-stream:stream"}

	for i, field := range attribFields {
		ava := pdata.NewAttributeValueArray()
		av := ava.ArrayVal()
		avs := av.AppendEmpty()
		pdata.NewAttributeValueString(attribVals[i]).CopyTo(avs)
		attr.Insert(field, ava)
	}

	d := Detector{provider: &mockMetaDataProvider{isV4: true}}
	got, _, err := d.Detect(context.TODO())

	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, internal.AttributesToMap(want.Attributes()), internal.AttributesToMap(got.Attributes()))
}

func Test_ecsDetectV3(t *testing.T) {
	os.Clearenv()
	os.Setenv(tmde3EnvVar, "endpoint")

	want := pdata.NewResource()
	attr := want.Attributes()
	attr.InsertString("cloud.provider", "aws")
	attr.InsertString("cloud.platform", "aws_ecs")
	attr.InsertString("aws.ecs.cluster.arn", "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster")
	attr.InsertString("aws.ecs.task.arn", "arn:aws:ecs:us-west-2:123456789123:task/123")
	attr.InsertString("aws.ecs.task.family", "family")
	attr.InsertString("aws.ecs.task.revision", "26")
	attr.InsertString("cloud.region", "us-west-2")
	attr.InsertString("cloud.availability_zone", "us-west-2a")
	attr.InsertString("cloud.account.id", "123456789123")

	d := Detector{provider: &mockMetaDataProvider{isV4: false}}
	got, _, err := d.Detect(context.TODO())

	assert.Nil(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, internal.AttributesToMap(want.Attributes()), internal.AttributesToMap(got.Attributes()))
}

func createTestContainer(isV4 bool) Container {
	c := Container{
		DockerID:    "123",
		Type:        "NORMAL",
		KnownStatus: "RUNNING",
	}

	if isV4 {
		c.LogDriver = "awslogs"
		c.ContainerARN = "arn:aws:ecs"
		c.LogOptions = LogData{LogGroup: "group", Region: "us-east-1", Stream: "stream"}
	}

	return c
}
