// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	v1 "k8s.io/api/core/v1"
)

type nodeInfo struct {
	name        string
	conditions  []*NodeCondition
	capacity    v1.ResourceList
	allocatable v1.ResourceList
}

type NodeCondition struct {
	Type   v1.NodeConditionType
	Status v1.ConditionStatus
}
