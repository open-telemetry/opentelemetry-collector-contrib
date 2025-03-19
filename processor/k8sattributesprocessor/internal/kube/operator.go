// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"regexp"
	"strings"

	"golang.org/x/exp/slices"

	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	v1 "k8s.io/api/core/v1"
)

type AutomaticRules struct {
	Enabled            bool     `mapstructure:"enabled"`
	Labels             bool     `mapstructure:"well_known_labels"`
	AnnotationPrefixes []string `mapstructure:"annotation_prefixes"`
	Exclude            []string `mapstructure:"exclude"`
}

func (r *AutomaticRules) IsEnabled(key string) bool {
	b := r.Enabled && !slices.Contains(r.Exclude, key)
	return b
}

func (r *AutomaticRules) NeedContainer() bool {
	return r.IsEnabled(conventions.AttributeServiceName) ||
		r.IsEnabled(conventions.AttributeServiceInstanceID) ||
		r.IsEnabled(conventions.AttributeServiceVersion)
}

const DefaultAnnotationPrefix = "resource.opentelemetry.io/"

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
	conventions.AttributeK8SDeploymentName,
	conventions.AttributeK8SReplicaSetName,
	conventions.AttributeK8SStatefulSetName,
	conventions.AttributeK8SDaemonSetName,
	conventions.AttributeK8SCronJobName,
	conventions.AttributeK8SJobName,
	conventions.AttributeK8SPodName,
}

func AutomaticAnnotationRule(prefix string) FieldExtractionRule {
	return FieldExtractionRule{
		Name:                 "$1",
		KeyRegex:             regexp.MustCompile(`^` + prefix + `(.+)$`),
		HasKeyRegexReference: true,
		From:                 MetadataFromPod,
	}
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
