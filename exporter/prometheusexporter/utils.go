// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.25.0"
)

func resourceSignature(attributes pcommon.Map) string {
	job, _ := extractJob(attributes)
	instance, _ := extractInstance(attributes)
	if job == "" || instance == "" {
		return ""
	}

	return job + separatorString + instance
}

func extractInstance(attributes pcommon.Map) (string, bool) {
	// Map service.instance.id to instance
	if inst, ok := attributes.Get(string(conventions.ServiceInstanceIDKey)); ok {
		return inst.AsString(), true
	}
	return "", false
}

func extractJob(attributes pcommon.Map) (string, bool) {
	// Map service.name + service.namespace to job
	if serviceName, ok := attributes.Get(string(conventions.ServiceNameKey)); ok {
		job := serviceName.AsString()
		if serviceNamespace, ok := attributes.Get(string(conventions.ServiceNamespaceKey)); ok {
			job = fmt.Sprintf("%s/%s", serviceNamespace.AsString(), job)
		}
		return job, true
	}
	return "", false
}
