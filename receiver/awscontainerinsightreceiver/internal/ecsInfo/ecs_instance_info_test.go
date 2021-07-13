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

type MockHostInfo struct{}

func (mi *MockHostInfo) GetInstanceIP() string {
	return "0.0.0.0"
}
func (mi *MockHostInfo) GetinstanceIPReadyC() chan bool {
	readyC := make(chan bool)
	return readyC
}

func TestECSInstanceInfo(t *testing.T) {

	var ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	instanceReadyC := make(chan bool)
	hostIPProvider := &MockHostInfo{}

	data, err := ioutil.ReadFile("./test/ecsinfo/clusterinfo")

	mockHTTP := &MockHTTPClient{
		responseData: data,
		err:          err,
	}

	//normal case
	ecsinstanceinfo := newECSInstanceInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, instanceReadyC)

	assert.NotNil(t, ecsinstanceinfo)

	<-instanceReadyC

	assert.Equal(t, "cluster_name", ecsinstanceinfo.GetClusterName())
	assert.Equal(t, "container_instance_id", ecsinstanceinfo.GetContainerInstanceID())

	//failed to get data

	data = nil

	err = errors.New("")

	mockHTTP = &MockHTTPClient{
		responseData: data,
		err:          err,
	}
	ecsinstanceinfo = newECSInstanceInfo(ctx, hostIPProvider, time.Minute, zap.NewNop(), mockHTTP, instanceReadyC)

	assert.NotNil(t, ecsinstanceinfo)

	assert.Equal(t, "", ecsinstanceinfo.GetClusterName())
	assert.Equal(t, "", ecsinstanceinfo.GetContainerInstanceID())

}
