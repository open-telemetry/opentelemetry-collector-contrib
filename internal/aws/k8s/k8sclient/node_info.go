// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	v1 "k8s.io/api/core/v1"
)

type nodeInfo struct {
	conditions []*nodeCondition
}

type nodeCondition struct {
	Type   v1.NodeConditionType
	Status v1.ConditionStatus
}
