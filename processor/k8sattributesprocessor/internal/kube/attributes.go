// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kube // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor/internal/kube"

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

// AttributesFunction is a function type that takes a key and value and returns an attribute.KeyValue.
// This is the signature used by semantic convention functions like conventions.K8SPodLabel.
type AttributesFunction func(key, val string) attribute.KeyValue

// K8SPodLabels returns an attribute KeyValue for pod labels using the plural form.
// Helper functions for plural forms (labels/annotations) that follow the same pattern as semconv functions.
// These are used for backward compatibility with historical attribute naming.
// From historical reasons some of workloads are using `*.labels.*` and `*.annotations.*` instead of
// `*.label.*` and `*.annotation.*`
// Semantic conventions define `*.label.*` and `*.annotation.*`
// More information - https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/37957
func K8SPodLabels(key, val string) attribute.KeyValue {
	return attribute.String(fmt.Sprintf("k8s.pod.labels.%s", key), val)
}

// K8SPodAnnotations returns an attribute KeyValue for pod annotations using the plural form
func K8SPodAnnotations(key, val string) attribute.KeyValue {
	return attribute.String(fmt.Sprintf("k8s.pod.annotations.%s", key), val)
}

// K8SNodeLabels returns an attribute KeyValue for node labels using the plural form
func K8SNodeLabels(key, val string) attribute.KeyValue {
	return attribute.String(fmt.Sprintf("k8s.node.labels.%s", key), val)
}

// K8SNodeAnnotations returns an attribute KeyValue for node annotations using the plural form
func K8SNodeAnnotations(key, val string) attribute.KeyValue {
	return attribute.String(fmt.Sprintf("k8s.node.annotations.%s", key), val)
}

// K8SNamespaceLabels returns an attribute KeyValue for namespace labels using the plural form
func K8SNamespaceLabels(key, val string) attribute.KeyValue {
	return attribute.String(fmt.Sprintf("k8s.namespace.labels.%s", key), val)
}

// K8SNamespaceAnnotations returns an attribute KeyValue for namespace annotations using the plural form
func K8SNamespaceAnnotations(key, val string) attribute.KeyValue {
	return attribute.String(fmt.Sprintf("k8s.namespace.annotations.%s", key), val)
}
