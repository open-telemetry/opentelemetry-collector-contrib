// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slowsqlconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/slowsqlconnector"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventionsv131 "go.opentelemetry.io/otel/semconv/v1.31.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

const (
	serviceNameKey        = string(conventions.ServiceNameKey)
	dbSystemKey           = string(conventions.DBSystemNameKey)
	statementExecDuration = conventionsv131.DBClientOperationDurationName
	spanKindKey           = "span.kind"    // OpenTelemetry non-standard constant.
	spanNameKey           = "span.name"    // OpenTelemetry non-standard constant.
	statusCodeKey         = "status.code"  // OpenTelemetry non-standard constant.
	dbStatementKey        = "db.statement" // OpenTelemetry non-standard constant.
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

func findAttributeValue(key string, attributes ...pcommon.Map) (string, bool) {
	for _, attr := range attributes {
		if v, ok := attr.Get(key); ok {
			return v.AsString(), true
		}
	}
	return "", false
}
