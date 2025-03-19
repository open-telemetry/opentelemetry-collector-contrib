// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.26.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
)

var prefix = metadata.Type.String()

// CollectorInstanceInfo holds the attributes that could uniquely identify
// the current collector instance. These attributes are initialized from the
// telemetry settings. The CollectorInstanceInfo can copy these attributes,
// with a given prefix, to a provided map.
type CollectorInstanceInfo struct {
	size              int
	serviceInstanceID string
	serviceName       string
	serviceNamespace  string
}

func NewCollectorInstanceInfo(
	set component.TelemetrySettings,
) *CollectorInstanceInfo {
	var info CollectorInstanceInfo
	for k, v := range set.Resource.Attributes().All() {
		switch k {
		case semconv.AttributeServiceInstanceID:
			if str := v.Str(); str != "" {
				info.serviceInstanceID = str
				info.size++
			}
		case semconv.AttributeServiceName:
			if str := v.Str(); str != "" {
				info.serviceName = str
				info.size++
			}
		case semconv.AttributeServiceNamespace:
			if str := v.Str(); str != "" {
				info.serviceNamespace = str
				info.size++
			}
		}
	}
	return &info
}

// Size returns the max number of attributes that defines a collector's
// instance information. Can be used to presize the attributes.
func (info CollectorInstanceInfo) Size() int {
	return info.size
}

func (info CollectorInstanceInfo) Copy(to pcommon.Map) {
	to.EnsureCapacity(info.Size())
	if info.serviceInstanceID != "" {
		to.PutStr(keyWithPrefix(semconv.AttributeServiceInstanceID), info.serviceInstanceID)
	}
	if info.serviceName != "" {
		to.PutStr(keyWithPrefix(semconv.AttributeServiceName), info.serviceName)
	}
	if info.serviceNamespace != "" {
		to.PutStr(keyWithPrefix(semconv.AttributeServiceNamespace), info.serviceNamespace)
	}
}

func keyWithPrefix(key string) string {
	return prefix + "." + key
}
