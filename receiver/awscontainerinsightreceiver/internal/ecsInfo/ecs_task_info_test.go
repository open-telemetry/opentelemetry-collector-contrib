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
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type MockHttpClient struct {
	maxRetries              int
	backoffRetryBaseInMills int
	client                  *http.Client
}

func (m *MockHttpClient) Request(endpoint string, ctx context.Context, logger *zap.Logger) ([]byte, error) {
	endpoint = "./test/ecsinfo/taskinfo"

	data, err := ioutil.ReadFile(endpoint)

	if err != nil {
		return nil, err
	}
	return data, nil
}

func TestECSTaskInfo(t *testing.T) {

	ctx := context.Background()

	taskReadyC := make(chan bool)
	hostIPProvider := &MockHostInfo{}
	mockHttp := &MockHttpClient{}

	ecsTaskinfo := newECSTaskInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHttp, taskReadyC)

	assert.NotNil(t, ecsTaskinfo)

	<-taskReadyC

	assert.Equal(t, int64(1), ecsTaskinfo.getRunningTaskCount())

	assert.NotEmpty(t, ecsTaskinfo.getRunningTasksInfo())

}
