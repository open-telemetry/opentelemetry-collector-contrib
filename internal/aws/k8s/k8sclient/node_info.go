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
	InstanceType string
	Labels       map[Label]int8
}

type NodeCondition struct {
	Type   v1.NodeConditionType
	Status v1.ConditionStatus
}

type Label int8

const (
	SageMakerNodeHealthStatus Label = iota
)

func (ct Label) String() string {
	return [...]string{"sagemaker.amazonaws.com/node-health-status"}[ct]
}
