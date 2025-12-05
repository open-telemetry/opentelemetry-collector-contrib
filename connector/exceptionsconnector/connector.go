// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

const (
	serviceNameKey         = string(conventions.ServiceNameKey)
	exceptionTypeKey       = string(conventions.ExceptionTypeKey)
	exceptionMessageKey    = string(conventions.ExceptionMessageKey)
	exceptionStacktraceKey = string(conventions.ExceptionStacktraceKey)
	// TODO(marctc): formalize these constants in the OpenTelemetry specification.
	spanKindKey   = "span.kind"   // OpenTelemetry non-standard constant.
	spanNameKey   = "span.name"   // OpenTelemetry non-standard constant.
	statusCodeKey = "status.code" // OpenTelemetry non-standard constant.
	eventNameExc  = "exception"   // OpenTelemetry non-standard constant.
)

func newDimensions(cfgDims []Dimension) []pdatautil.Dimension {
	if len(cfgDims) == 0 {
		return nil
	}
	dims := make([]pdatautil.Dimension, len(cfgDims))
	for i := range cfgDims {
		dims[i].Name = cfgDims[i].Name
		if cfgDims[i].Default != nil {
			val := pcommon.NewValueStr(*cfgDims[i].Default)
			dims[i].Value = &val
		}
	}
	return dims
}
