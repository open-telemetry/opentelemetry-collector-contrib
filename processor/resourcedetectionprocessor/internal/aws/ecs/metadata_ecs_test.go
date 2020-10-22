package ecs

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	taskMeta = `{"Cluster":"myCluster",
		"TaskARN":"arn:aws:ecs:ap-southeast-1:123456789123:task/123",
		"Family":"myFamily",
		"LaunchType":"ec2",
		"AvailabilityZone":"ap-southeast-1a",
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

func Test_ecsMetadata_fetchTask(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, taskMeta)
	}))
	defer ts.Close()

	md := ecsMetadataProviderImpl{}
	fetchResp, err := md.fetchTaskMetaData(ts.URL)

	assert.Nil(t, err)
	assert.Equal(t, "myCluster", fetchResp.Cluster)
	assert.Equal(t, "arn:aws:ecs:ap-southeast-1:123456789123:task/123", fetchResp.TaskARN)
	assert.Equal(t, "myFamily", fetchResp.Family)
	assert.Equal(t, "ec2", fetchResp.LaunchType)
	assert.Equal(t, "ap-southeast-1a", fetchResp.AvailabilityZone)
	assert.Empty(t, fetchResp.Containers)
}

func Test_ecsMetadata_fetchContainer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, containerMeta)
	}))
	defer ts.Close()

	md := ecsMetadataProviderImpl{}
	fetchResp, err := md.fetchContainerMetaData(ts.URL)

	assert.Nil(t, err)
	assert.NotNil(t, fetchResp)
	assert.Equal(t, "abcdef12345", fetchResp.DockerId)
	assert.Equal(t, "arn:aws:ecs:ap-southeast-1:123456789123:container/123", fetchResp.ContainerARN)
	assert.Equal(t, "RUNNING", fetchResp.KnownStatus)
	assert.Equal(t, "awslogs", fetchResp.LogDriver)
	assert.Equal(t, "helloworld", fetchResp.LogOptions.LogGroup)
	assert.Equal(t, "ap-southeast-1", fetchResp.LogOptions.Region)
	assert.Equal(t, "logs/main/456", fetchResp.LogOptions.Stream)
}
