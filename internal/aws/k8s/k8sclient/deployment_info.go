// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

type DeploymentInfo struct {
	Name      string
	Namespace string
	Spec      *DeploymentSpec
	Status    *DeploymentStatus
}

type DeploymentSpec struct {
	Replicas uint32
}

type DeploymentStatus struct {
	Replicas            uint32
	ReadyReplicas       uint32
	AvailableReplicas   uint32
	UnavailableReplicas uint32
}
