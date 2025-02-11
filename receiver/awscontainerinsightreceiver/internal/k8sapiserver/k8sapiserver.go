// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapiserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8sapiserver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sutil"
)

var HyperPodConditions = []k8sutil.HyperPodConditionType{
	k8sutil.UnschedulablePendingReplacement,
	k8sutil.UnschedulablePendingReboot,
	k8sutil.Unschedulable,
	k8sutil.Schedulable,
}

const HyperPodNodePrefix = "hyperpod-"

// eventBroadcaster is adpated from record.EventBroadcaster
type eventBroadcaster interface {
	// StartRecordingToSink starts sending events received from this EventBroadcaster to the given
	// sink. The return value can be ignored or used to stop recording, if desired.
	StartRecordingToSink(sink record.EventSink) watch.Interface
	// StartLogging starts sending events received from this EventBroadcaster to the given logging
	// function. The return value can be ignored or used to stop recording, if desired.
	StartLogging(logf func(format string, args ...any)) watch.Interface
	// NewRecorder returns an EventRecorder that can be used to send events to this EventBroadcaster
	// with the event source set to the given event source.
	NewRecorder(scheme *runtime.Scheme, source v1.EventSource) record.EventRecorderLogger
}

type K8sClient interface {
	GetClientSet() kubernetes.Interface
	GetEpClient() k8sclient.EpClient
	GetNodeClient() k8sclient.NodeClient
	GetPodClient() k8sclient.PodClient
	GetDeploymentClient() k8sclient.DeploymentClient
	GetDaemonSetClient() k8sclient.DaemonSetClient
	GetStatefulSetClient() k8sclient.StatefulSetClient
	GetReplicaSetClient() k8sclient.ReplicaSetClient
	ShutdownNodeClient()
	ShutdownPodClient()
}

// K8sAPIServer is a struct that produces metrics from kubernetes api server
type K8sAPIServer struct {
	nodeName                  string // get the value from downward API
	logger                    *zap.Logger
	clusterNameProvider       clusterNameProvider
	cancel                    context.CancelFunc
	leaderElection            *LeaderElection
	addFullPodNameMetricLabel bool
	includeEnhancedMetrics    bool
}

type clusterNameProvider interface {
	GetClusterName() string
}

type Option func(*K8sAPIServer)

// NewK8sAPIServer creates a k8sApiServer which can generate cluster-level metrics
func NewK8sAPIServer(cnp clusterNameProvider, logger *zap.Logger, leaderElection *LeaderElection, addFullPodNameMetricLabel bool, includeEnhancedMetrics bool, options ...Option) (*K8sAPIServer, error) {
	k := &K8sAPIServer{
		logger:                    logger,
		clusterNameProvider:       cnp,
		leaderElection:            leaderElection,
		addFullPodNameMetricLabel: addFullPodNameMetricLabel,
		includeEnhancedMetrics:    includeEnhancedMetrics,
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
	result = append(result, k.getStatefulSetMetrics(clusterName, timestampNs)...)
	result = append(result, k.getReplicaSetMetrics(clusterName, timestampNs)...)
	result = append(result, k.getPendingPodStatusMetrics(clusterName, timestampNs)...)

	if k.includeEnhancedMetrics {
		result = append(result, k.getHyperPodResiliencyMetrics(clusterName, timestampNs)...)
	}

	return result
}

func (k *K8sAPIServer) getClusterMetrics(clusterName, timestampNs string) pmetric.Metrics {
	fields := map[string]any{
		"cluster_failed_node_count": k.leaderElection.nodeClient.ClusterFailedNodeCount(),
		"cluster_node_count":        k.leaderElection.nodeClient.ClusterNodeCount(),
	}

	namespaceMap := k.leaderElection.podClient.NamespaceToRunningPodNum()
	clusterPodCount := 0
	for _, value := range namespaceMap {
		clusterPodCount += value
	}
	fields["cluster_number_of_running_pods"] = clusterPodCount

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
		fields := map[string]any{
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
		fields := map[string]any{
			ci.ReplicasDesired:           deployment.Spec.Replicas,              // replicas_desired
			ci.ReplicasReady:             deployment.Status.ReadyReplicas,       // replicas_ready
			ci.StatusReplicasAvailable:   deployment.Status.AvailableReplicas,   // status_replicas_available
			ci.StatusReplicasUnavailable: deployment.Status.UnavailableReplicas, // status_replicas_unavailable
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
		fields := map[string]any{
			ci.StatusReplicasAvailable:   daemonSet.Status.NumberAvailable,        // status_replicas_available
			ci.StatusReplicasUnavailable: daemonSet.Status.NumberUnavailable,      // status_replicas_unavailable
			ci.ReplicasDesired:           daemonSet.Status.DesiredNumberScheduled, // replicas_desired
			ci.ReplicasReady:             daemonSet.Status.CurrentNumberScheduled, // replicas_ready
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
		fields := map[string]any{
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

func (k *K8sAPIServer) getStatefulSetMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	statefulSets := k.leaderElection.statefulSetClient.StatefulSetInfos()
	for _, statefulSet := range statefulSets {
		fields := map[string]any{
			ci.ReplicasDesired:         statefulSet.Spec.Replicas,            // replicas_desired
			ci.ReplicasReady:           statefulSet.Status.ReadyReplicas,     // replicas_ready
			ci.StatusReplicasAvailable: statefulSet.Status.AvailableReplicas, // status_replicas_available
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterStatefulSet,
			ci.Timestamp:      timestampNs,
			ci.PodNameKey:     statefulSet.Name,
			ci.K8sNamespace:   statefulSet.Namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes[ci.NodeNameKey] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		metrics = append(metrics, md)
	}
	return metrics
}

func (k *K8sAPIServer) getReplicaSetMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	replicaSets := k.leaderElection.replicaSetClient.ReplicaSetInfos()
	for _, replicaSet := range replicaSets {
		fields := map[string]any{
			ci.ReplicasDesired:         replicaSet.Spec.Replicas,            // replicas_desired
			ci.ReplicasReady:           replicaSet.Status.ReadyReplicas,     // replicas_ready
			ci.StatusReplicasAvailable: replicaSet.Status.AvailableReplicas, // status_replicas_available
		}
		attributes := map[string]string{
			ci.ClusterNameKey: clusterName,
			ci.MetricType:     ci.TypeClusterReplicaSet,
			ci.Timestamp:      timestampNs,
			ci.PodNameKey:     replicaSet.Name,
			ci.K8sNamespace:   replicaSet.Namespace,
			ci.Version:        "0",
		}
		if k.nodeName != "" {
			attributes[ci.NodeNameKey] = k.nodeName
		}
		attributes[ci.SourcesKey] = "[\"apiserver\"]"
		md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
		metrics = append(metrics, md)
	}
	return metrics
}

// Statues and conditions for all pods assigned to a node are determined in podstore.go. Given Pending pods do not have a node allocated to them, we need to fetch their details from the K8s API Server here.
func (k *K8sAPIServer) getPendingPodStatusMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	podsList := k.leaderElection.podClient.PodInfos()
	podKeyToServiceNamesMap := k.leaderElection.epClient.PodKeyToServiceNames()

	for _, podInfo := range podsList {
		if podInfo.Phase == v1.PodPending {
			fields := map[string]any{}

			if k.includeEnhancedMetrics {
				addPodStatusMetrics(fields, podInfo)
				addPodConditionMetrics(fields, podInfo)
			}

			attributes := map[string]string{
				ci.ClusterNameKey: clusterName,
				ci.MetricType:     ci.TypePod,
				ci.Timestamp:      timestampNs,
				ci.PodNameKey:     podInfo.Name,
				ci.K8sNamespace:   podInfo.Namespace,
				ci.Version:        "0",
			}

			podKey := k8sutil.CreatePodKey(podInfo.Namespace, podInfo.Name)
			if serviceList, ok := podKeyToServiceNamesMap[podKey]; ok {
				if len(serviceList) > 0 {
					attributes[ci.TypeService] = serviceList[0]
				}
			}

			attributes[ci.PodStatus] = string(v1.PodPending)
			attributes["k8s.node.name"] = "pending"

			kubernetesBlob := map[string]any{}
			k.getKubernetesBlob(podInfo, kubernetesBlob, attributes)
			if k.nodeName != "" {
				kubernetesBlob["host"] = k.nodeName
			}
			if len(kubernetesBlob) > 0 {
				kubernetesInfo, err := json.Marshal(kubernetesBlob)
				if err != nil {
					k.logger.Warn("Error parsing kubernetes blob for pod metrics")
				} else {
					attributes[ci.Kubernetes] = string(kubernetesInfo)
				}
			}
			attributes[ci.SourcesKey] = "[\"apiserver\"]"
			md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
			metrics = append(metrics, md)
		}
	}
	return metrics
}

// TODO this is duplicated code from podstore.go, move this to a common package to re-use
func (k *K8sAPIServer) getKubernetesBlob(pod *k8sclient.PodInfo, kubernetesBlob map[string]any, attributes map[string]string) {
	var owners []any
	podName := ""
	for _, owner := range pod.OwnerReferences {
		if owner.Kind != "" && owner.Name != "" {
			kind := owner.Kind
			name := owner.Name
			if owner.Kind == ci.ReplicaSet {
				rsToDeployment := k.leaderElection.replicaSetClient.ReplicaSetToDeployment()
				if parent := rsToDeployment[owner.Name]; parent != "" {
					kind = ci.Deployment
					name = parent
				} else if parent := parseDeploymentFromReplicaSet(owner.Name); parent != "" {
					kind = ci.Deployment
					name = parent
				}
			} else if owner.Kind == ci.Job {
				if parent := parseCronJobFromJob(owner.Name); parent != "" {
					kind = ci.CronJob
					name = parent
				} else if !k.addFullPodNameMetricLabel {
					name = getJobNamePrefix(name)
				}
			}
			owners = append(owners, map[string]string{"owner_kind": kind, "owner_name": name})

			if podName == "" {
				if owner.Kind == ci.StatefulSet {
					podName = pod.Name
				} else if owner.Kind == ci.DaemonSet || owner.Kind == ci.Job ||
					owner.Kind == ci.ReplicaSet || owner.Kind == ci.ReplicationController {
					podName = name
				}
			}
		}
	}

	if len(owners) > 0 {
		kubernetesBlob["pod_owners"] = owners
	}

	labels := make(map[string]string)
	for k, v := range pod.Labels {
		labels[k] = v
	}
	if len(labels) > 0 {
		kubernetesBlob["labels"] = labels
	}
	kubernetesBlob["namespace_name"] = pod.Namespace
	kubernetesBlob["pod_id"] = pod.UID

	// if podName is not set according to a well-known controllers, then set it to its own name
	if podName == "" {
		if strings.HasPrefix(pod.Name, KubeProxy) && !k.addFullPodNameMetricLabel {
			podName = KubeProxy
		} else {
			podName = pod.Name
		}
	}

	attributes[ci.PodNameKey] = podName
	if k.addFullPodNameMetricLabel {
		attributes[ci.FullPodNameKey] = pod.Name
		kubernetesBlob["pod_name"] = pod.Name
	}
}

func (k *K8sAPIServer) getHyperPodResiliencyMetrics(clusterName, timestampNs string) []pmetric.Metrics {
	var metrics []pmetric.Metrics
	nodeInfos := k.leaderElection.nodeClient.NodeInfos()
	for nodeName, labels := range k.leaderElection.nodeClient.NodeToLabelsMap() {
		if nodeInfo, ok := nodeInfos[nodeName]; ok {
			if isHyperPodNode(nodeInfo.InstanceType) {
				fields := map[string]any{}
				attributes := map[string]string{
					ci.ClusterNameKey: clusterName,
					ci.MetricType:     ci.TypeHyperPodNode,
					ci.Timestamp:      timestampNs,
					ci.Version:        "0",
				}

				for _, condition := range HyperPodConditions {
					if count, ok := isLabelSet(int8(condition), labels, k8sclient.SageMakerNodeHealthStatus); ok {
						fields[ci.MetricName(ci.TypeHyperPodNode, ci.HyperPodConditionToMetric[condition.String()])] = count
					}
				}
				attributes[ci.InstanceID] = strings.TrimPrefix(nodeName, HyperPodNodePrefix)
				attributes[ci.NodeNameKey] = nodeName
				md := ci.ConvertToOTLPMetrics(fields, attributes, k.logger)
				metrics = append(metrics, md)
			}
		}
	}
	return metrics
}

// Shutdown stops the k8sApiServer
func (k *K8sAPIServer) Shutdown() error {
	if k.cancel != nil {
		k.cancel()
	}
	return nil
}
