// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"strings"

	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	v1 "k8s.io/api/core/v1"
)

// AutomaticLabelRules has rules where the last entry wins
var AutomaticLabelRules = []FieldExtractionRule{
	{
		Name: "service.name",
		Key:  "app.kubernetes.io/name",
		From: MetadataFromPod,
	},
	{
		Name: "service.name",
		Key:  "app.kubernetes.io/instance",
		From: MetadataFromPod,
	},
	{
		Name: "service.version",
		Key:  "app.kubernetes.io/version",
		From: MetadataFromPod,
	},
}

var serviceNamePrecedence = []string{
	string(conventions.K8SDeploymentNameKey),
	string(conventions.K8SReplicaSetNameKey),
	string(conventions.K8SStatefulSetNameKey),
	string(conventions.K8SDaemonSetNameKey),
	string(conventions.K8SCronJobNameKey),
	string(conventions.K8SJobNameKey),
	string(conventions.K8SPodNameKey),
}

func AutomaticServiceName(containerName string, names map[string]string) string {
	for _, k := range serviceNamePrecedence {
		if v, ok := names[k]; ok {
			return v
		}
	}
	return containerName
}

func automaticServiceInstanceID(pod *v1.Pod, containerName string) string {
	resNames := []string{pod.Namespace, pod.Name, containerName}
	return strings.Join(resNames, ".")
}
