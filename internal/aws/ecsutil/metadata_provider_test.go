// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsutil

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/ecsutiltest"
)

var _ Client = &mockClient{}

type mockClient struct {
	response string
	retErr   bool
}

func (mc *mockClient) Get(_ string) ([]byte, error) {
	if mc.retErr {
		return nil, errors.New("fake error")
	}

	return []byte(mc.response), nil
}

func Test_ecsMetadata_fetchTask(t *testing.T) {
	mockRestClient := NewRestClientFromClient(&mockClient{response: string(ecsutiltest.TaskMetadataTestResponse), retErr: false})
	md := ecsMetadataProviderImpl{logger: zap.NewNop(), client: mockRestClient}
	fetchResp, err := md.FetchTaskMetadata()

	assert.NoError(t, err)
	assert.NotNil(t, fetchResp)
	assert.Equal(t, "test200", fetchResp.Cluster)
	assert.Equal(t, "arn:aws:ecs:us-west-2:803860917211:task/test200/d22aaa11bf0e4ab19c2c940a1cbabbee", fetchResp.TaskARN)
	assert.Equal(t, "three-nginx", fetchResp.Family)
	assert.Equal(t, "ec2", fetchResp.LaunchType)
	assert.Equal(t, "us-west-2a", fetchResp.AvailabilityZone)
	assert.Equal(t, "1", fetchResp.Revision)
	assert.Equal(t, 3, len(fetchResp.Containers))
}

func Test_ecsMetadata_fetchContainer(t *testing.T) {
	mockRestClient := NewRestClientFromClient(&mockClient{response: string(ecsutiltest.ContainerMetadataTestResponse), retErr: false})
	md := ecsMetadataProviderImpl{logger: zap.NewNop(), client: mockRestClient}
	fetchResp, err := md.FetchContainerMetadata()

	assert.Nil(t, err)
	assert.NotNil(t, fetchResp)
	assert.Equal(t, "325c979aea914acd93be2fdd2429e1d9-3811061257", fetchResp.DockerID)
	assert.Equal(t, "arn:aws:ecs:us-east-1:123456789123:an-image/123", fetchResp.ContainerARN)
	assert.Equal(t, "RUNNING", fetchResp.KnownStatus)
	assert.Equal(t, "awslogs", fetchResp.LogDriver)
	assert.Equal(t, "helloworld", fetchResp.LogOptions.LogGroup)
	assert.Equal(t, "us-east-1", fetchResp.LogOptions.Region)
	assert.Equal(t, "logs/main/456", fetchResp.LogOptions.Stream)
}

func Test_ecsMetadata_returnsError(t *testing.T) {
	mockRestClient := NewRestClientFromClient(&mockClient{response: "task-metadata", retErr: true})
	md := ecsMetadataProviderImpl{logger: zap.NewNop(), client: mockRestClient}
	fetchResp, err := md.FetchContainerMetadata()

	assert.Nil(t, fetchResp)
	assert.EqualError(t, err, "fake error")
}
