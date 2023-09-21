// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

type ReplicaSetInfo struct {
	Name      string
	Namespace string
	Owners    []*ReplicaSetOwner
	Spec      *ReplicaSetSpec
	Status    *ReplicaSetStatus
}

type ReplicaSetOwner struct {
	kind string
	name string
}

type ReplicaSetSpec struct {
	Replicas uint32
}

type ReplicaSetStatus struct {
	Replicas          uint32
	AvailableReplicas uint32
	ReadyReplicas     uint32
}
