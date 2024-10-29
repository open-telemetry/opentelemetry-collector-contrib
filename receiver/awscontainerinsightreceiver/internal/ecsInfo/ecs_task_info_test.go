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

func TestECSTaskInfoSuccess(t *testing.T) {

	ctx := context.Background()

	taskReadyC := make(chan bool)
	hostIPProvider := &MockHostInfo{}

	data, err := os.ReadFile("./test/ecsinfo/taskinfo")

	respBody := string(data)
	httpResponse := &http.Response{
		StatusCode:    200,
		Body:          io.NopCloser(bytes.NewBufferString(respBody)),
		Header:        make(http.Header),
		ContentLength: 5 * 1024,
	}

	mockHTTP := &mockHTTPClient{
		response: httpResponse,
		err:      err,
	}

	ecsTaskinfo := newECSTaskInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, taskReadyC)

	assert.NotNil(t, ecsTaskinfo)

	<-taskReadyC

	assert.Equal(t, int64(1), ecsTaskinfo.getRunningTaskCount())

	assert.NotEmpty(t, ecsTaskinfo.getRunningTasksInfo())

}

func TestECSTaskInfoFail(t *testing.T) {
	ctx := context.Background()
	err := errors.New("")
	taskReadyC := make(chan bool)

	hostIPProvider := &MockHostInfo{}

	respBody := ""

	httpResponse := &http.Response{
		Status:        "Bad Request",
		StatusCode:    400,
		Body:          io.NopCloser(bytes.NewBufferString(respBody)),
		Header:        make(http.Header),
		ContentLength: 5 * 1024,
	}

	mockHTTP := &mockHTTPClient{
		response: httpResponse,
		err:      err,
	}
	ecsTaskinfo := newECSTaskInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, taskReadyC)
	assert.NotNil(t, ecsTaskinfo)
	assert.Equal(t, int64(0), ecsTaskinfo.getRunningTaskCount())
	assert.Empty(t, ecsTaskinfo.getRunningTasksInfo())

	data, err := os.ReadFile("./test/ecsinfo/taskinfo_wrong")
	body := string(data)
	httpResponse.Body = io.NopCloser(bytes.NewBufferString(body))
	mockHTTP = &mockHTTPClient{
		response: httpResponse,
		err:      err,
	}
	ecsTaskinfo = newECSTaskInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, taskReadyC)
	assert.NotNil(t, ecsTaskinfo)
	assert.Equal(t, int64(0), ecsTaskinfo.getRunningTaskCount())
	assert.Empty(t, ecsTaskinfo.getRunningTasksInfo())

}
