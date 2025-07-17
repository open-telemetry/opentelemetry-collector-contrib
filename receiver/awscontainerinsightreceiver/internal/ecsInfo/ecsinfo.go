// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/ecsInfo"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
)

const defaultTimeout = 1 * time.Second

type hostIPProvider interface {
	GetInstanceIP() string
	GetInstanceIPReadyC() chan bool
}

type EcsInfo struct {
	logger                *zap.Logger
	refreshInterval       time.Duration
	cancel                context.CancelFunc
	hostIPProvider        hostIPProvider
	isTaskInfoReadyC      chan bool
	isContainerInfoReadyC chan bool

	isCgroupReadyC          chan bool // close of this channel indicates cgroup is initialized. It is used only in test
	taskInfoTestReadyC      chan bool // close of this channel indicates taskinfo is initialized. It is used only in test
	containerInfoTestReadyC chan bool // close of this channel indicates container info is initialized. It is used only in test

	httpClient            doer
	containerInstanceInfo containerInstanceInfoProvider
	ecsTaskInfo           ecsTaskInfoProvider
	cgroup                cgroupScannerProvider

	containerInstanceInfoCreator func(context.Context, hostIPProvider, time.Duration, *zap.Logger, doer, chan bool) containerInstanceInfoProvider
	ecsTaskInfoCreator           func(context.Context, hostIPProvider, time.Duration, *zap.Logger, doer, chan bool) ecsTaskInfoProvider
	cgroupScannerCreator         func(context.Context, *zap.Logger, ecsTaskInfoProvider, containerInstanceInfoProvider, time.Duration) cgroupScannerProvider
}

func (e *EcsInfo) GetRunningTaskCount() int64 {
	if e.ecsTaskInfo != nil {
		return e.ecsTaskInfo.getRunningTaskCount()
	}
	return 0
}

func (e *EcsInfo) GetCPUReserved() int64 {
	if e.cgroup != nil {
		return e.cgroup.getCPUReserved()
	}
	return 0
}

func (e *EcsInfo) GetMemReserved() int64 {
	if e.cgroup != nil {
		return e.cgroup.getMemReserved()
	}
	return 0
}

func (e *EcsInfo) GetContainerInstanceID() string {
	if e.containerInstanceInfo != nil {
		return e.containerInstanceInfo.GetContainerInstanceID()
	}
	return ""
}

func (e *EcsInfo) GetClusterName() string {
	if e.containerInstanceInfo != nil {
		return e.containerInstanceInfo.GetClusterName()
	}
	return ""
}

type ecsInfoOption func(*EcsInfo)

// New creates a k8sApiServer which can generate cluster-level metrics
func NewECSInfo(refreshInterval time.Duration, hostIPProvider hostIPProvider, host component.Host, settings component.TelemetrySettings, options ...ecsInfoOption) (*EcsInfo, error) {
	setting := confighttp.ClientConfig{
		Timeout: defaultTimeout,
	}
	ctx, cancel := context.WithCancel(context.Background())

	client, err := setting.ToClient(ctx, host, settings)
	if err != nil {
		settings.Logger.Warn("Failed to create a http client for ECS info!")
		cancel()
		return nil, err
	}

	ecsInfo := &EcsInfo{
		logger:                       settings.Logger,
		hostIPProvider:               hostIPProvider,
		refreshInterval:              refreshInterval,
		httpClient:                   client,
		cancel:                       cancel,
		containerInstanceInfoCreator: newECSInstanceInfo,
		ecsTaskInfoCreator:           newECSTaskInfo,
		cgroupScannerCreator:         newCGroupScannerForContainer,
		isTaskInfoReadyC:             make(chan bool),
		isContainerInfoReadyC:        make(chan bool),
		isCgroupReadyC:               make(chan bool),
		taskInfoTestReadyC:           make(chan bool),
		containerInfoTestReadyC:      make(chan bool),
	}

	for _, opt := range options {
		opt(ecsInfo)
	}

	go ecsInfo.initContainerInfo(ctx)

	go ecsInfo.initTaskInfo(ctx)

	go ecsInfo.initCgroupScanner(ctx)

	return ecsInfo, nil
}

func (e *EcsInfo) initContainerInfo(ctx context.Context) {
	<-e.hostIPProvider.GetInstanceIPReadyC()

	e.logger.Info("instance ip is ready and begin initializing ecs container info")

	e.containerInstanceInfo = e.containerInstanceInfoCreator(ctx, e.hostIPProvider, e.refreshInterval, e.logger, e.httpClient, e.isContainerInfoReadyC)
	close(e.containerInfoTestReadyC)
}

func (e *EcsInfo) initTaskInfo(ctx context.Context) {
	<-e.hostIPProvider.GetInstanceIPReadyC()

	e.logger.Info("instance ip is ready and begin initializing ecs task info")

	e.ecsTaskInfo = e.ecsTaskInfoCreator(ctx, e.hostIPProvider, e.refreshInterval, e.logger, e.httpClient, e.isTaskInfoReadyC)

	close(e.taskInfoTestReadyC)
}

func (e *EcsInfo) initCgroupScanner(ctx context.Context) {
	<-e.isContainerInfoReadyC
	<-e.isTaskInfoReadyC

	e.logger.Info("info ready and begin getting info")

	e.cgroup = e.cgroupScannerCreator(ctx, e.logger, e.ecsTaskInfo, e.containerInstanceInfo, e.refreshInterval)

	close(e.isCgroupReadyC)
}

// Shutdown stops the ecs Info
func (e *EcsInfo) Shutdown() {
	e.cancel()
}
