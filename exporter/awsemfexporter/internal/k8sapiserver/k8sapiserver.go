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
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

// const (
// 	lockName = "otel-container-insight-clusterleader"
// )

type K8sClient interface {
	GetClientSet() kubernetes.Interface
	GetEpClient() k8sclient.EpClient
	GetPodClient() k8sclient.PodClient
}

// K8sAPIServer is a struct that produces metrics from kubernetes api server
type K8sAPIServer struct {
	logger      *zap.Logger
	clusterName string

	k8sClient K8sClient //*k8sclient.K8sClient
	epClient  k8sclient.EpClient
	podClient k8sclient.PodClient
}

type k8sAPIServerOption func(*K8sAPIServer)

// New creates a k8sApiServer which can generate cluster-level metrics
func New(clusterName string, logger *zap.Logger, options ...k8sAPIServerOption) (*K8sAPIServer, error) {
	k := &K8sAPIServer{
		logger:      logger,
		clusterName: clusterName,
		k8sClient:   k8sclient.Get(logger),
	}

	for _, opt := range options {
		opt(k)
	}

	if k.k8sClient == nil {
		return nil, errors.New("failed to start k8sapiserver because k8sclient is nil")
	}
	k.epClient = k.k8sClient.GetEpClient()
	k.podClient = k.k8sClient.GetPodClient()

	return k, nil
}

// GetMetrics returns an array of metrics
func (k *K8sAPIServer) GetMetrics() []pdata.Metrics {
	var result []pdata.Metrics

	k.logger.Info("collect data from K8s API Server...")
	timestampNs := strconv.FormatInt(time.Now().UnixNano(), 10)

	for service, podNum := range k.epClient.ServiceToPodNum() {
		fields := map[string]interface{}{
			"service_number_of_running_pods": podNum,
		}
		attributes := map[string]string{
			ci.ClusterNameKey: k.clusterName,
			ci.MetricType:     ci.TypeClusterService,
			ci.Timestamp:      timestampNs,
			ci.TypeService:    service.ServiceName,
			ci.K8sNamespace:   service.Namespace,
			ci.Version:        "0",
		}

		attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\",\"service_name\":\"%s\"}",
			service.Namespace, service.ServiceName)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		result = append(result, md)
	}

	for namespace, podNum := range k.podClient.NamespaceToRunningPodNum() {
		fields := map[string]interface{}{
			"namespace_number_of_running_pods": podNum,
		}
		attributes := map[string]string{
			ci.ClusterNameKey: k.clusterName,
			ci.MetricType:     ci.TypeClusterNamespace,
			ci.Timestamp:      timestampNs,
			ci.K8sNamespace:   namespace,
			ci.Version:        "0",
		}

		attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\"}", namespace)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		result = append(result, md)
	}

	return result
}
