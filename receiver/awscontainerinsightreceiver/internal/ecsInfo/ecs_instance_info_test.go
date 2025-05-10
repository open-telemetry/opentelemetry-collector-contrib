// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsinfo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type MockHostInfo struct{}

func (mi *MockHostInfo) GetInstanceIP() string {
	return "0.0.0.0"
}
func (mi *MockHostInfo) GetInstanceIPReadyC() chan bool {
	readyC := make(chan bool)
	return readyC
}

func TestECSInstanceInfo(t *testing.T) {
	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	instanceReadyC := make(chan bool)
	hostIPProvider := &MockHostInfo{}

	data, err := os.ReadFile("./test/ecsinfo/clusterinfo")
	respBody := string(data)

	httpResponse := &http.Response{
		StatusCode:    http.StatusOK,
		Body:          io.NopCloser(bytes.NewBufferString(respBody)),
		Header:        make(http.Header),
		ContentLength: 5 * 1024,
	}

	mockHTTP := &mockHTTPClient{
		response: httpResponse,
		err:      err,
	}

	// normal case
	ecsinstanceinfo := newECSInstanceInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, instanceReadyC)

	assert.NotNil(t, ecsinstanceinfo)

	<-instanceReadyC

	assert.Equal(t, "cluster_name", ecsinstanceinfo.GetClusterName())
	assert.Equal(t, "container_instance_id", ecsinstanceinfo.GetContainerInstanceID())

	// failed to get data

	err = errors.New("")

	httpResponse = &http.Response{
		Status:        "Bad Request",
		StatusCode:    http.StatusBadRequest,
		Body:          io.NopCloser(bytes.NewBufferString(respBody)),
		Header:        make(http.Header),
		ContentLength: 5 * 1024,
	}

	mockHTTP = &mockHTTPClient{
		response: httpResponse,
		err:      err,
	}
	ecsinstanceinfo = newECSInstanceInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, instanceReadyC)

	assert.NotNil(t, ecsinstanceinfo)

	assert.Equal(t, "", ecsinstanceinfo.GetClusterName())
	assert.Equal(t, "", ecsinstanceinfo.GetContainerInstanceID())
}
