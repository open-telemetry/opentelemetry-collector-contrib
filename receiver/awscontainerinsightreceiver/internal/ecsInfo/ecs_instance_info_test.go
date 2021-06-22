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

type MockHttpClientforInstance struct {
	maxRetries              int
	backoffRetryBaseInMills int
	client                  *http.Client
}

func (m *MockHttpClientforInstance) Request(endpoint string, ctx context.Context, logger *zap.Logger) ([]byte, error) {
	endpoint = "./test/ecsinfo/clusterinfo"

	data, err := ioutil.ReadFile(endpoint)

	if err != nil {
		return nil, err
	}
	return data, nil
}

type MockHostInfo struct{}

func (mi *MockHostInfo) GetInstanceIp() string {
	return "0.0.0.0"
}
func (mi *MockHostInfo) GetinstanceIpReadyC() chan bool {
	readyC := make(chan bool)
	return readyC
}

func TestECSInstanceInfo(t *testing.T) {

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	instanceReadyC := make(chan bool)
	hostIPProvider := &MockHostInfo{}
	mockHttp := &MockHttpClientforInstance{}

	ecsinstanceinfo := newECSInstanceInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHttp, instanceReadyC)

	assert.NotNil(t, ecsinstanceinfo)

	<-instanceReadyC

	assert.Equal(t, "cluster_name", ecsinstanceinfo.GetClusterName())
	assert.Equal(t, "container_instance_id", ecsinstanceinfo.GetContainerInstanceId())
}
