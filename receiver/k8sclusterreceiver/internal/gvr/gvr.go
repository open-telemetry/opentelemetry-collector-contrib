// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gvr // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvr"

import "k8s.io/apimachinery/pkg/runtime/schema"

// Kubernetes group version resources
var (
	HierarchicalResourceQuota = schema.GroupVersionResource{Group: "hnc.x-k8s.io", Version: "v1alpha2", Resource: "hierarchicalresourcequotas"}
)
