// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"

// Resource label keys.
const (
	// TODO: Remove after switch to new Metrics definition
	K8sType       = "k8s"
	ContainerType = "container"

	// Resource labels keys for UID.
	K8sKeyNamespaceUID             = "k8s.namespace.uid"
	K8sKeyReplicationControllerUID = "k8s.replicationcontroller.uid"
	K8sKeyHPAUID                   = "k8s.hpa.uid"
	K8sKeyResourceQuotaUID         = "k8s.resourcequota.uid"
	K8sKeyClusterResourceQuotaUID  = "openshift.clusterquota.uid"

	// Resource labels keys for Name.
	K8sKeyReplicationControllerName = "k8s.replicationcontroller.name"
	K8sKeyHPAName                   = "k8s.hpa.name"
	K8sKeyResourceQuotaName         = "k8s.resourcequota.name"
	K8sKeyClusterResourceQuotaName  = "openshift.clusterquota.name"

	// Kubernetes resource kinds
	K8sKindCronJob               = "CronJob"
	K8sKindDaemonSet             = "DaemonSet"
	K8sKindDeployment            = "Deployment"
	K8sKindJob                   = "Job"
	K8sKindReplicationController = "ReplicationController"
	K8sKindReplicaSet            = "ReplicaSet"
	K8sStatefulSet               = "StatefulSet"
)

// Keys for K8s metadata
const (
	K8sKeyWorkLoadKind = "k8s.workload.kind"
	K8sKeyWorkLoadName = "k8s.workload.name"

	K8sServicePrefix = "k8s.service."
)
