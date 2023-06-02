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
	assert.Equal(t, 0, len(ecsTaskinfo.getRunningTasksInfo()))

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
	assert.Equal(t, 0, len(ecsTaskinfo.getRunningTasksInfo()))

}
