// Copyright The OpenTelemetry Authors
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

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

// K8sAPIServer is a struct that produces metrics from kubernetes api server
type K8sAPIServer struct {
	nodeName            string // get the value from downward API
	logger              *zap.Logger
	clusterNameProvider clusterNameProvider
	cancel              context.CancelFunc
	leaderElection      *LeaderElection
}

type clusterNameProvider interface {
	GetClusterName() string
}

type Option func(*K8sAPIServer)

// NewK8sAPIServer creates a k8sApiServer which can generate cluster-level metrics
func NewK8sAPIServer(cnp clusterNameProvider, logger *zap.Logger, leaderElection *LeaderElection, options ...Option) (*K8sAPIServer, error) {

	k := &K8sAPIServer{
		logger:              logger,
		clusterNameProvider: cnp,
		leaderElection:      leaderElection,
	}

	for _, opt := range options {
		opt(k)
	}

	if k.leaderElection == nil {
		return nil, errors.New("cannot start k8sapiserver, leader election is nil")
	}

	_, k.cancel = context.WithCancel(context.Background())

	k.nodeName = os.Getenv("HOST_NAME")
	if k.nodeName == "" {
		return nil, errors.New("environment variable HOST_NAME is not set in k8s deployment config")
	}

	return k, nil
}

// GetMetrics returns an array of metrics
func (k *K8sAPIServer) GetMetrics() []pmetric.Metrics {
	var result []pmetric.Metrics

	// don't generate any metrics if the current collector is not the leader
	if !k.leaderElection.leading {
		return result
	}

	// don't emit metrics if the cluster name is not detected
	clusterName := k.clusterNameProvider.GetClusterName()
	if clusterName == "" {
		k.logger.Warn("Failed to detect cluster name. Drop all metrics")
		return result
	}

	k.logger.Info("collect data from K8s API Server...")
	timestampNs := strconv.FormatInt(time.Now().UnixNano(), 10)

	result = append(result, k.getClusterMetrics(clusterName, timestampNs))
	result = append(result, k.getNamespaceMetrics(clusterName, timestampNs)...)
	result = append(result, k.getDeploymentMetrics(clusterName, timestampNs)...)
	result = append(result, k.getDaemonSetMetrics(clusterName, timestampNs)...)
	result = append(result, k.getServiceMetrics(clusterName, timestampNs)...)

	return result
}

func (k *K8sAPIServer) getClusterMetrics(clusterName, timestampNs string) pmetric.Metrics {
	fields := map[string]interface{}{
		"cluster_failed_node_count": k.leaderElection.nodeClient.ClusterFailedNodeCount(),
		"cluster_node_count":        k.leaderElection.nodeClient.ClusterNodeCount(),
	}
	attributes := map[string]string{
		ci.ClusterNameKey: clusterName,
		ci.MetricType:     ci.TypeCluster,
		ci.Timestamp:      timestampNs,
		ci.Version:        "0",
	}
	if k.nodeName != "" {
		attributes["NodeName"] = k.nodeName
	}
	attributes[ci.SourcesKey] = "[\"apiserver\"]"
	return ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
}

func (k *K8sAPIServer) getNamespaceMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	for namespace, podNum := range k.leaderElection.podClient.NamespaceToRunningPodNum() {
		fields := map[string]interface{}{
			"namespace_number_of_running_pods": podNum,
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterNamespace,
			ci.Timestamp:      timestampNs,
			ci.K8sNamespace:   namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes["NodeName"] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\"}", namespace)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		metrics = append(metrics, md)
	}
	return metrics
}

func (k *K8sAPIServer) getDeploymentMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	deployments := k.leaderElection.deploymentClient.DeploymentInfos()
	for _, deployment := range deployments {
		fields := map[string]interface{}{
			ci.MetricName(ci.TypeClusterDeployment, ci.SpecReplicas):              deployment.Spec.Replicas,              // deployment_spec_replicas
			ci.MetricName(ci.TypeClusterDeployment, ci.StatusReplicas):            deployment.Status.Replicas,            // deployment_status_replicas
			ci.MetricName(ci.TypeClusterDeployment, ci.StatusReplicasAvailable):   deployment.Status.AvailableReplicas,   // deployment_status_replicas_available
			ci.MetricName(ci.TypeClusterDeployment, ci.StatusReplicasUnavailable): deployment.Status.UnavailableReplicas, // deployment_status_replicas_unavailable
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterDeployment,
			ci.Timestamp:      timestampNs,
			ci.PodNameKey:     deployment.Name,
			ci.K8sNamespace:   deployment.Namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes[ci.NodeNameKey] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		// attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\",\"deployment_name\":\"%s\"}",
		//	deployment.Namespace, deployment.Name)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		metrics = append(metrics, md)
	}
	return metrics
}

func (k *K8sAPIServer) getDaemonSetMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	daemonSets := k.leaderElection.daemonSetClient.DaemonSetInfos()
	for _, daemonSet := range daemonSets {
		fields := map[string]interface{}{
			ci.MetricName(ci.TypeClusterDaemonSet, ci.StatusNumberAvailable):        daemonSet.Status.NumberAvailable,        // daemonset_status_number_available
			ci.MetricName(ci.TypeClusterDaemonSet, ci.StatusNumberUnavailable):      daemonSet.Status.NumberUnavailable,      // daemonset_status_number_unavailable
			ci.MetricName(ci.TypeClusterDaemonSet, ci.StatusDesiredNumberScheduled): daemonSet.Status.DesiredNumberScheduled, // daemonset_status_desired_number_scheduled
			ci.MetricName(ci.TypeClusterDaemonSet, ci.StatusCurrentNumberScheduled): daemonSet.Status.CurrentNumberScheduled, // daemonset_status_current_number_scheduled
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterDaemonSet,
			ci.Timestamp:      timestampNs,
			ci.PodNameKey:     daemonSet.Name,
			ci.K8sNamespace:   daemonSet.Namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes[ci.NodeNameKey] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		// attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\",\"daemonset_name\":\"%s\"}",
		//	daemonSet.Namespace, daemonSet.Name)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		metrics = append(metrics, md)
	}
	return metrics
}

func (k *K8sAPIServer) getServiceMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	for service, podNum := range k.leaderElection.epClient.ServiceToPodNum() {
		fields := map[string]interface{}{
			"service_number_of_running_pods": podNum,
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterService,
			ci.Timestamp:      timestampNs,
			ci.TypeService:    service.ServiceName,
			ci.K8sNamespace:   service.Namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes["NodeName"] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		attributes[ci.Kubernetes] = fmt.Sprintf("{\"namespace_name\":\"%s\",\"service_name\":\"%s\"}",
			service.Namespace, service.ServiceName)
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		metrics = append(metrics, md)
	}
	return metrics
}

// Shutdown stops the k8sApiServer
func (k *K8sAPIServer) Shutdown() {
	if k.cancel != nil {
		k.cancel()
	}
}
