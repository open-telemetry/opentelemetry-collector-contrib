// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/ecsInfo"

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
)

type containerInstanceInfoProvider interface {
	GetClusterName() string
	GetContainerInstanceID() string
}

type Requester interface {
	Request(ctx context.Context, path string) ([]byte, error)
}

type containerInstanceInfo struct {
	logger                   *zap.Logger
	httpClient               doer
	refreshInterval          time.Duration
	ecsAgentEndpointProvider hostIPProvider
	clusterName              string
	containerInstanceID      string
	readyC                   chan bool
	sync.RWMutex
}

type ContainerInstance struct {
	Cluster              string
	ContainerInstanceArn string
}

func newECSInstanceInfo(ctx context.Context, ecsAgentEndpointProvider hostIPProvider,
	refreshInterval time.Duration, logger *zap.Logger, httpClient doer, readyC chan bool) containerInstanceInfoProvider {
	cii := &containerInstanceInfo{
		logger:                   logger,
		httpClient:               httpClient,
		refreshInterval:          refreshInterval,
		ecsAgentEndpointProvider: ecsAgentEndpointProvider,
		readyC:                   readyC,
	}

	shouldRefresh := func() bool {
		// stop the refresh once we get instance ID and cluster name successfully
		return cii.GetClusterName() == "" || cii.GetContainerInstanceID() == ""
	}

	go host.RefreshUntil(ctx, cii.refresh, cii.refreshInterval, shouldRefresh, 0)

	return cii
}

func (cii *containerInstanceInfo) refresh(ctx context.Context) {
	containerInstance := &ContainerInstance{}
	cii.logger.Info("Fetch instance id and type from ec2 metadata")
	resp, err := request(ctx, cii.getECSAgentEndpoint(), cii.httpClient)
	if err != nil {
		cii.logger.Warn("Failed to call ecsagent endpoint, error: ", zap.Error(err))
	}

	err = json.Unmarshal(resp, containerInstance)
	if err != nil {
		cii.logger.Warn("Failed to unmarshal: ", zap.Error(err))
		cii.logger.Debug("Resp content is " + string(resp))
	}

	cluster := containerInstance.Cluster
	instanceID, err := GetContainerInstanceIDFromArn(containerInstance.ContainerInstanceArn)

	if err != nil {
		cii.logger.Warn("Failed to get instance id from arn, error: ", zap.Error(err))
	}

	cii.Lock()
	cii.clusterName = cluster
	cii.containerInstanceID = instanceID
	defer cii.Unlock()

	// notify cgroups that the clustername and instanceID is ready
	if cii.clusterName != "" && cii.containerInstanceID != "" && !isClosed(cii.readyC) {
		close(cii.readyC)
	}
}

func (cii *containerInstanceInfo) GetClusterName() string {
	cii.RLock()
	defer cii.RUnlock()
	return cii.clusterName
}

func (cii *containerInstanceInfo) GetContainerInstanceID() string {
	cii.RLock()
	defer cii.RUnlock()
	return cii.containerInstanceID
}

func (cii *containerInstanceInfo) getECSAgentEndpoint() string {
	return fmt.Sprintf(ecsAgentEndpoint, cii.ecsAgentEndpointProvider.GetInstanceIP())
}
