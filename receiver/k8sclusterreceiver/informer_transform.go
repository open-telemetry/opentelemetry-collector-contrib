// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/demonset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/deployment"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/jobs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/node"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/pod"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replicaset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/service"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/statefulset"
)

// transformObject transforms the k8s object by removing the data that is not utilized by the receiver.
// Only highly utilized objects are transformed here while others are kept as is.
func transformObject(object any) (any, error) {
	switch o := object.(type) {
	case *corev1.Pod:
		return pod.Transform(o), nil
	case *corev1.Node:
		return node.Transform(o), nil
	case *appsv1.ReplicaSet:
		return replicaset.Transform(o), nil
	case *batchv1.Job:
		return jobs.Transform(o), nil
	case *appsv1.Deployment:
		return deployment.Transform(o), nil
	case *appsv1.DaemonSet:
		return demonset.Transform(o), nil
	case *appsv1.StatefulSet:
		return statefulset.Transform(o), nil
	case *corev1.Service:
		return service.Transform(o), nil
	}
	return object, nil
}
