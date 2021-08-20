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
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	taskMeta = `{"Cluster":"myCluster",
		"TaskARN":"arn:aws:ecs:ap-southeast-1:123456789123:task/123",
		"Family":"myFamily",
		"LaunchType":"ec2",
		"AvailabilityZone":"ap-southeast-1a",
		"Revision":"26",
		"Containers": []
	}`

	containerMeta = `{
		"DockerId":"abcdef12345",
		"Type":"NORMAL",
		"KnownStatus":"RUNNING",
		"LogDriver":"awslogs",
		"LogOptions": {
			"awslogs-group":"helloworld",
			"awslogs-region":"ap-southeast-1",
			"awslogs-stream":"logs/main/456"
		},
		"ContainerARN":"arn:aws:ecs:ap-southeast-1:123456789123:container/123"
	}`
)

type mockClient struct {
	response string
	retErr   bool
}

func (mc *mockClient) Do(*http.Request) (*http.Response, error) {
	if mc.retErr {
		return nil, errors.New("fake error")
	}

	r := ioutil.NopCloser(bytes.NewReader([]byte(mc.response)))
	return &http.Response{
		StatusCode: 200,
		Body:       r,
	}, nil
}

func Test_ecsMetadata_fetchTask(t *testing.T) {
	md := ecsMetadataProviderImpl{logger: zap.NewNop(), client: &mockClient{response: taskMeta, retErr: false}}
	fetchResp, err := md.fetchTaskMetaData("url")

	assert.Nil(t, err)
	assert.Equal(t, "myCluster", fetchResp.Cluster)
	assert.Equal(t, "arn:aws:ecs:ap-southeast-1:123456789123:task/123", fetchResp.TaskARN)
	assert.Equal(t, "myFamily", fetchResp.Family)
	assert.Equal(t, "ec2", fetchResp.LaunchType)
	assert.Equal(t, "ap-southeast-1a", fetchResp.AvailabilityZone)
	assert.Equal(t, "26", fetchResp.Revision)
	assert.Empty(t, fetchResp.Containers)
}

func Test_ecsMetadata_fetchContainer(t *testing.T) {
	md := ecsMetadataProviderImpl{logger: zap.NewNop(), client: &mockClient{response: containerMeta, retErr: false}}
	fetchResp, err := md.fetchContainerMetaData("url")

	assert.Nil(t, err)
	assert.NotNil(t, fetchResp)
	assert.Equal(t, "abcdef12345", fetchResp.DockerID)
	assert.Equal(t, "arn:aws:ecs:ap-southeast-1:123456789123:container/123", fetchResp.ContainerARN)
	assert.Equal(t, "RUNNING", fetchResp.KnownStatus)
	assert.Equal(t, "awslogs", fetchResp.LogDriver)
	assert.Equal(t, "helloworld", fetchResp.LogOptions.LogGroup)
	assert.Equal(t, "ap-southeast-1", fetchResp.LogOptions.Region)
	assert.Equal(t, "logs/main/456", fetchResp.LogOptions.Stream)
}

func Test_ecsMetadata_returnsError(t *testing.T) {
	md := ecsMetadataProviderImpl{logger: zap.NewNop(), client: &mockClient{response: "{}", retErr: true}}
	fetchResp, err := md.fetchContainerMetaData("url")

	assert.Nil(t, fetchResp)
	assert.NotNil(t, err)
}
