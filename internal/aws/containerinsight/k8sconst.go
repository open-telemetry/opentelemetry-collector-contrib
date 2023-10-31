// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package containerinsight // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"

// define constants that are used for EKS Container Insights only
const (
	EKS            = "eks"
	KubeSecurePort = "10250"

	// attribute names
	Kubernetes       = "kubernetes"
	K8sNamespace     = "Namespace"
	PodIDKey         = "PodId"
	PodNameKey       = "PodName"
	FullPodNameKey   = "FullPodName"
	K8sPodNameKey    = "K8sPodName"
	ContainerNamekey = "ContainerName"
	ContainerIDkey   = "ContainerId"

	PodStatus       = "pod_status"
	ContainerStatus = "container_status"

	ContainerStatusReason          = "container_status_reason"
	ContainerLastTerminationReason = "container_last_termination_reason"

	// Pod Owners
	ReplicaSet            = "ReplicaSet"
	ReplicationController = "ReplicationController"
	StatefulSet           = "StatefulSet"
	DaemonSet             = "DaemonSet"
	Deployment            = "Deployment"
	Job                   = "Job"
	CronJob               = "CronJob"
)
