// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"regexp"
	"strings"

	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	v1 "k8s.io/api/core/v1"
)

type OperatorRules struct {
	Enabled bool `mapstructure:"enabled"`
	Labels  bool `mapstructure:"labels"`
}

var OperatorAnnotationRule = FieldExtractionRule{
	Name:                 "$1",
	KeyRegex:             regexp.MustCompile(`^resource.opentelemetry.io/(.+)$`),
	HasKeyRegexReference: true,
	From:                 MetadataFromPod,
}

var OperatorLabelRules = []FieldExtractionRule{
	{
		Name: "service.name",
		Key:  "app.kubernetes.io/name",
		From: MetadataFromPod,
	},
	{
		Name: "service.version",
		Key:  "app.kubernetes.io/version",
		From: MetadataFromPod,
	},
	{
		Name: "service.namespace",
		Key:  "app.kubernetes.io/part-of",
		From: MetadataFromPod,
	},
}

var serviceNamePrecedence = []string{
	conventions.AttributeK8SDeploymentName,
	conventions.AttributeK8SReplicaSetName,
	conventions.AttributeK8SStatefulSetName,
	conventions.AttributeK8SDaemonSetName,
	conventions.AttributeK8SCronJobName,
	conventions.AttributeK8SJobName,
	conventions.AttributeK8SPodName,
}

func OperatorServiceName(containerName string, names map[string]string) string {
	for _, k := range serviceNamePrecedence {
		if v, ok := names[k]; ok {
			return v
		}
	}
	return containerName
}

func operatorServiceInstanceID(pod *v1.Pod, containerName string) string {
	resNames := []string{pod.Namespace, pod.Name, containerName}
	return strings.Join(resNames, ".")
}
