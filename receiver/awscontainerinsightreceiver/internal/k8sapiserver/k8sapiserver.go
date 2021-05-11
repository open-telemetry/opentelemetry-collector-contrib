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

package k8sapiserver

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// K8sAPIServer is a struct that produces metrics from kubernetes api server
type K8sAPIServer struct {
	logger              *zap.Logger
	clusterNameProvider clusterNameProvider
	cancel              context.CancelFunc
}

type clusterNameProvider interface {
	GetClusterName() string
}

// New creates a k8sApiServer which can generate cluster-level metrics
func New(clusterNameProvider clusterNameProvider, logger *zap.Logger) *K8sAPIServer {
	_, cancel := context.WithCancel(context.Background())
	k := &K8sAPIServer{
		logger:              logger,
		clusterNameProvider: clusterNameProvider,
		cancel:              cancel,
	}

	if err := k.start(); err != nil {
		k.logger.Warn("Fail to start k8sapiserver", zap.Error(err))
		return nil
	}

	return k
}

// GetMetrics returns an array of metrics
func (k *K8sAPIServer) GetMetrics() []pdata.Metrics {
	// TODO: add the logic to generate the metrics
	var result []pdata.Metrics
	return result
}

func (k *K8sAPIServer) start() error {
	//TODO: add implementation
	return nil
}

// Stop stops the k8sApiServer
func (k *K8sAPIServer) Stop() {
	if k.cancel != nil {
		k.cancel()
	}
}
