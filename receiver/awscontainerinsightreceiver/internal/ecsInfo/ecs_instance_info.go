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
	"time"

	httpClient "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight/httpclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

	"go.uber.org/zap"
)

type containerInstanceInfoProvider interface {
	GetClusterName() string
	GetContainerInstanceId() string
}

type HttpClientProvider interface {
	Request(path string, ctx context.Context, logger *zap.Logger) ([]byte, error)
}

type containerInstanceInfo struct {
	logger                   *zap.Logger
	httpClient               httpClient.HttpClientProvider
	refreshInterval          time.Duration
	ecsAgentEndpointProvider hostIPProvider
	clusterName              string
	containerInstanceId      string
	readyC                   chan bool
}

type ContainerInstance struct {
	Cluster              string
	ContainerInstanceArn string
}

type ecsInstanceInfoOption func(*containerInstanceInfo)

func newECSInstanceInfo(ctx context.Context, ecsAgentEndpointProvider hostIPProvider,
	refreshInterval time.Duration, logger *zap.Logger, httpClient httpClient.HttpClientProvider, readyC chan bool, options ...ecsInstanceInfoOption) containerInstanceInfoProvider {
	cii := &containerInstanceInfo{
		logger:                   logger,
		httpClient:               httpClient,
		refreshInterval:          refreshInterval,
		ecsAgentEndpointProvider: ecsAgentEndpointProvider,
		readyC:                   readyC,
	}

	for _, opt := range options {
		opt(cii)
	}

	shouldRefresh := func() bool {
		//stop the refresh once we get instance ID and type successfully
		return cii.clusterName == "" || cii.containerInstanceId == ""
	}
	go host.RefreshUntil(ctx, cii.refresh, cii.refreshInterval, shouldRefresh, 0)

	return cii
}

func (cii *containerInstanceInfo) refresh(ctx context.Context) {
	containerInstance := &ContainerInstance{}
	cii.logger.Info("Fetch instance id and type from ec2 metadata")
	resp, err := cii.httpClient.Request(cii.getECSAgentEndpoint(), ctx, cii.logger)
	if err != nil {
		cii.logger.Warn("Failed to call ecsagent endpoint, error: ", zap.Error(err))
	}

	err = json.Unmarshal(resp, containerInstance)
	if err != nil {
		cii.logger.Warn("Unable to parse resp from ecsagent endpoint, error: ", zap.Error(err))
		cii.logger.Warn("Resp content is " + string(resp))
	}

	cii.clusterName = containerInstance.Cluster
	cii.containerInstanceId, err = GetContainerInstanceIdFromArn(containerInstance.ContainerInstanceArn)
	if err != nil {
		cii.logger.Warn("Failed to get instance id from arn, error: ", zap.Error(err))
	}
	if cii.clusterName != "" && cii.containerInstanceId != "" {
		close(cii.readyC)
	}
}

func (cii *containerInstanceInfo) GetClusterName() string {
	return cii.clusterName
}

func (cii *containerInstanceInfo) GetContainerInstanceId() string {
	return cii.containerInstanceId
}

func (cii *containerInstanceInfo) getECSAgentEndpoint() string {
	return fmt.Sprintf(ecsAgentEndpoint, cii.ecsAgentEndpointProvider.GetInstanceIp())
}
