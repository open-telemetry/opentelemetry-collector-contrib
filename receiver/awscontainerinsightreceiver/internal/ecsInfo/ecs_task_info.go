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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	httpClient "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight/httpclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
)

type ecsTaskInfoProvider interface {
	getRunningTaskCount() int64
	getRunningTasksInfo() []ECSTask
}

type ECSContainer struct {
	DockerID string
}
type ECSTask struct {
	KnownStatus string
	ARN         string
	Containers  []ECSContainer
}

type ECSTasksInfo struct {
	Tasks []ECSTask
}

type taskInfo struct {
	logger                  *zap.Logger
	httpClient              httpClient.Requester
	refreshInterval         time.Duration
	ecsTaskEndpointProvider hostIPProvider
	runningTaskCount        int64
	runningTasksInfo        []ECSTask
	readyC                  chan bool
	sync.RWMutex
}

type taskInfoOption func(*taskInfo)

func newECSTaskInfo(ctx context.Context, ecsTaskEndpointProvider hostIPProvider,
	refreshInterval time.Duration, logger *zap.Logger, httpClient Requester, readyC chan bool, options ...taskInfoOption) ecsTaskInfoProvider {
	ti := &taskInfo{
		logger:                  logger,
		httpClient:              httpClient,
		refreshInterval:         refreshInterval,
		ecsTaskEndpointProvider: ecsTaskEndpointProvider,
		readyC:                  readyC,
	}
	for _, opt := range options {
		opt(ti)
	}
	shouldRefresh := func() bool {
		//stop the refresh once we get instance ID and type successfully
		return ti.runningTaskCount == 0 || len(ti.runningTasksInfo) == 0
	}
	go host.RefreshUntil(ctx, ti.refresh, ti.refreshInterval, shouldRefresh, 0)
	return ti
}

func (ti *taskInfo) getTasksInfo(ctx context.Context) (ecsTasksInfo *ECSTasksInfo) {
	ecsTasksInfo = &ECSTasksInfo{}
	resp, err := ti.httpClient.Request(ctx, ti.getECSAgentTaskInfoEndpoint(), ti.logger)
	if err != nil {
		ti.logger.Warn("Failed to call ecsagent taskinfo endpoint, error: ", zap.Error(err))
		return ecsTasksInfo
	}

	err = json.Unmarshal(resp, ecsTasksInfo)
	if err != nil {
		ti.logger.Warn("Unable to parse resp from ecsagent taskinfo endpoint, error:", zap.Error(err))
		ti.logger.Warn("D! resp content is %s" + string(resp))
	}
	return ecsTasksInfo
}

func (ti *taskInfo) refresh(ctx context.Context) {

	ecsTasksInfo := ti.getTasksInfo(ctx)
	runningTaskCount := int64(0)
	tasks := []ECSTask{}
	for _, task := range ecsTasksInfo.Tasks {
		if task.KnownStatus != taskStatusRunning {
			continue
		}
		tasks = append(tasks, task)
		runningTaskCount++
	}

	ti.Lock()
	defer ti.Unlock()
	ti.runningTaskCount = runningTaskCount
	ti.runningTasksInfo = tasks
	if len(ti.runningTasksInfo) != 0 && ti.runningTaskCount != 0 {
		close(ti.readyC)
	}

}

func (ti *taskInfo) getRunningTaskCount() int64 {
	ti.RLock()
	defer ti.RUnlock()
	return ti.runningTaskCount
}

func (ti *taskInfo) getRunningTasksInfo() []ECSTask {
	ti.RLock()
	defer ti.RUnlock()
	return ti.runningTasksInfo
}

func (ti *taskInfo) getECSAgentTaskInfoEndpoint() string {
	return fmt.Sprintf(ecsAgentTaskInfoEndpoint, ti.ecsTaskEndpointProvider.GetInstanceIP())
}
