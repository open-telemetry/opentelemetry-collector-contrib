// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	v1 "k8s.io/api/core/v1"
)

type NodeInfo struct {
	Name         string
	Conditions   []*NodeCondition
	Capacity     v1.ResourceList
	Allocatable  v1.ResourceList
	ProviderId   string
	InstanceType string
}

type NodeCondition struct {
	Type   v1.NodeConditionType
	Status v1.ConditionStatus
}
