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
	"time"

	httpClient "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight/httpclient"

	"go.uber.org/zap"
)

type ecsInfo interface {
	GetCpuReserved() float64
	GetMemReserved() float64
	GetRunningTaskCount() int64
	GetContainerInstanceId() string
	GetClusterName() string
}

type EcsInfo struct {
	logger                *zap.Logger
	hostIP                string
	runningTaskCount      int64
	cpuReserved           int64
	memReserved           int64
	refreshInterval       time.Duration
	cancel                context.CancelFunc
	hostIPProvider        hostIPProvider
	isTaskInfoReadyC      chan bool
	isContainerInfoReadyC chan bool

	httpClient            httpClient.HttpClientProvider
	containerInstanceInfo containerInstanceInfoProvider
	ecsTaskInfo           ecsTaskInfoProvider
	cgroup                cgroupScannerProvider

	containerInstanceInfoCreator func(context.Context, hostIPProvider, time.Duration, *zap.Logger, httpClient.HttpClientProvider, chan bool, ...ecsInstanceInfoOption) containerInstanceInfoProvider
	ecsTaskInfoCreator           func(context.Context, hostIPProvider, time.Duration, *zap.Logger, httpClient.HttpClientProvider, chan bool, ...taskInfoOption) ecsTaskInfoProvider
	cgroupScannerCreator         func(context.Context, *zap.Logger, ecsTaskInfoProvider, containerInstanceInfoProvider, time.Duration) cgroupScannerProvider
}

func (e *EcsInfo) GetRunningTaskCount() int64 {
	if e.ecsTaskInfo != nil {
		return e.ecsTaskInfo.getRunningTaskCount()
	}
	return 0
}

func (e *EcsInfo) GetCpuReserved() int64 {
	if e.cgroup != nil {
		return e.cgroup.getCpuReserved()
	}
	return 0
	//return e.cgroup.getCpuReserved()
}

func (e *EcsInfo) GetMemReserved() int64 {
	if e.cgroup != nil {
		return e.cgroup.getMemReserved()
	}
	return 0
	//return e.cgroup.getMemReserved()

}

func (e *EcsInfo) GetContainerInstanceId() string {
	if e.containerInstanceInfo != nil {
		return e.containerInstanceInfo.GetContainerInstanceId()
	}
	return ""
}

func (e *EcsInfo) GetClusterName() string {
	if e.containerInstanceInfo != nil {
		return e.containerInstanceInfo.GetClusterName()
	}
	return ""
}

type hostIPProvider interface {
	GetInstanceIp() string
	GetinstanceIpReadyC() chan bool
}

type ecsInfoOption func(*EcsInfo)

// New creates a k8sApiServer which can generate cluster-level metrics
func NewECSInfo(refreshInterval time.Duration, hostIPProvider hostIPProvider, logger *zap.Logger, options ...ecsInfoOption) (*EcsInfo, error) {

	ctx, cancel := context.WithCancel(context.Background())
	ecsInfo := &EcsInfo{
		logger:                       logger,
		hostIPProvider:               hostIPProvider,
		refreshInterval:              refreshInterval,
		httpClient:                   httpClient.New(),
		cancel:                       cancel,
		containerInstanceInfoCreator: newECSInstanceInfo,
		ecsTaskInfoCreator:           newECSTaskInfo,
		cgroupScannerCreator:         newCGroupScannerForContainer,
		isTaskInfoReadyC:             make(chan bool),
		isContainerInfoReadyC:        make(chan bool),
	}

	for _, opt := range options {
		opt(ecsInfo)
	}

	go ecsInfo.initContainerAndTaskInfo(ctx)

	go ecsInfo.initCgroupScanner(ctx)

	return ecsInfo, nil
}

func (e *EcsInfo) initContainerAndTaskInfo(ctx context.Context) {

	<-e.hostIPProvider.GetinstanceIpReadyC()

	e.logger.Info("instance ip is ready and begin initializing ecs task info and container info")

	e.containerInstanceInfo = e.containerInstanceInfoCreator(ctx, e.hostIPProvider, e.refreshInterval, e.logger, e.httpClient, e.isContainerInfoReadyC)
	e.ecsTaskInfo = e.ecsTaskInfoCreator(ctx, e.hostIPProvider, e.refreshInterval, e.logger, e.httpClient, e.isTaskInfoReadyC)
}

func (e *EcsInfo) initCgroupScanner(ctx context.Context) {

	<-e.isContainerInfoReadyC
	<-e.isTaskInfoReadyC

	e.logger.Info("info ready and begin getting info")

	e.cgroup = e.cgroupScannerCreator(ctx, e.logger, e.ecsTaskInfo, e.containerInstanceInfo, e.refreshInterval)
}

// Shutdown stops the ecs Info
func (e *EcsInfo) Shutdown() {
	e.cancel()
}
