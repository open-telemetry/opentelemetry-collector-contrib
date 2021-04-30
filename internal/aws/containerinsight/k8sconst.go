// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package containerinsight

//define constants that are used for EKS Container Insights only
const (
	EKS            = "eks"
	KubeSecurePort = "10250"
	BearerToken    = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec to suppress lint error G101: Potential hardcoded credentials
	CAFile         = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	//attribute names
	Kubernetes       = "kubernetes"
	K8sNamespace     = "Namespace"
	PodIDKey         = "PodId"
	PodNameKey       = "PodName"
	K8sPodNameKey    = "K8sPodName"
	ContainerNamekey = "ContainerName"
	ContainerIDkey   = "ContainerId"
	PodOwnersKey     = "PodOwners"

	PodStatus       = "pod_status"
	ContainerStatus = "container_status"

	ContainerStatusReason          = "container_status_reason"
	ContainerLastTerminationReason = "container_last_termination_reason"

	//Pod Owners
	ReplicaSet            = "ReplicaSet"
	ReplicationController = "ReplicationController"
	StatefulSet           = "StatefulSet"
	DaemonSet             = "DaemonSet"
	Deployment            = "Deployment"
	Job                   = "Job"
	CronJob               = "CronJob"
)
