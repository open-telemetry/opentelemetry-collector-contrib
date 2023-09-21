// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

type StatefulSetInfo struct {
	Name      string
	Namespace string
	Spec      *StatefulSetSpec
	Status    *StatefulSetStatus
}

type StatefulSetSpec struct {
	Replicas uint32
}

type StatefulSetStatus struct {
	Replicas          uint32
	AvailableReplicas uint32
	ReadyReplicas     uint32
}
