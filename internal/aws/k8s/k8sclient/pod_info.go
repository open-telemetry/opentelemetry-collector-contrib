// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclient // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"

import (
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodInfo struct {
	Name            string
	Namespace       string
	Uid             string
	Labels          map[string]string
	OwnerReferences []metaV1.OwnerReference
	Phase           v1.PodPhase
	Conditions      []v1.PodCondition
}
