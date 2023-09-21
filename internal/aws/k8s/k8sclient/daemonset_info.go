// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

type DaemonSetInfo struct {
	Name      string
	Namespace string
	Status    *DaemonSetStatus
}

type DaemonSetStatus struct {
	NumberAvailable        uint32
	NumberUnavailable      uint32
	DesiredNumberScheduled uint32
	CurrentNumberScheduled uint32
}
