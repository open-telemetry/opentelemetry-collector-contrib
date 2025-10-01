// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func extractInstance(attributes pcommon.Map) (string, bool) {
	// Map service.instance.id to instance
	if inst, ok := attributes.Get(conventions.AttributeServiceInstanceID); ok {
		return inst.AsString(), true
	}
	return "", false
}

func extractJob(attributes pcommon.Map) (string, bool) {
	// Map service.namespace + service.name to job
	if serviceName, ok := attributes.Get(conventions.AttributeServiceName); ok {
		job := serviceName.AsString()
		if serviceNamespace, ok := attributes.Get(conventions.AttributeServiceNamespace); ok {
			job = fmt.Sprintf("%s/%s", serviceNamespace.AsString(), job)
		}
		return job, true
	}
	return "", false
}
