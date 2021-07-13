// Copyright  OpenTelemetry Authors
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
	"context"
	"errors"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestECSTaskInfoSuccess(t *testing.T) {

	ctx := context.Background()

	taskReadyC := make(chan bool)
	hostIPProvider := &MockHostInfo{}

	data, err := ioutil.ReadFile("./test/ecsinfo/taskinfo")

	mockHTTP := &MockHTTPClient{
		responseData: data,
		err:          err,
	}

	ecsTaskinfo := newECSTaskInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, taskReadyC)

	assert.NotNil(t, ecsTaskinfo)

	<-taskReadyC

	assert.Equal(t, int64(1), ecsTaskinfo.getRunningTaskCount())

	assert.NotEmpty(t, ecsTaskinfo.getRunningTasksInfo())

}

func TestECSTaskInfoFail(t *testing.T) {
	ctx := context.Background()
	var data []byte
	data = nil
	err := errors.New("")
	taskReadyC := make(chan bool)

	hostIPProvider := &MockHostInfo{}
	mockHTTP := &MockHTTPClient{
		responseData: data,
		err:          err,
	}
	ecsTaskinfo := newECSTaskInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, taskReadyC)
	assert.NotNil(t, ecsTaskinfo)
	assert.Equal(t, int64(0), ecsTaskinfo.getRunningTaskCount())
	assert.Equal(t, 0, len(ecsTaskinfo.getRunningTasksInfo()))

	data, err = ioutil.ReadFile("./test/ecsinfo/taskinfo_wrong")

	mockHTTP = &MockHTTPClient{
		responseData: data,
		err:          err,
	}
	ecsTaskinfo = newECSTaskInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, taskReadyC)
	assert.NotNil(t, ecsTaskinfo)
	assert.Equal(t, int64(0), ecsTaskinfo.getRunningTaskCount())
	assert.Equal(t, 0, len(ecsTaskinfo.getRunningTasksInfo()))

}
