// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/constants"

const (
	// Kubernetes resource kinds
	K8sKindCronJob               = "CronJob"
	K8sKindDaemonSet             = "DaemonSet"
	K8sKindDeployment            = "Deployment"
	K8sKindJob                   = "Job"
	K8sKindReplicationController = "ReplicationController"
	K8sKindReplicaSet            = "ReplicaSet"
	K8sKindService               = "Service"
	K8sStatefulSet               = "StatefulSet"

	// Keys for K8s metadata
	K8sKeyWorkLoadKind = "k8s.workload.kind"
	K8sKeyWorkLoadName = "k8s.workload.name"

	K8sServicePrefix = "k8s.service."
)
