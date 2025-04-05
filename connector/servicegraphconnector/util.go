// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

func findServiceName(attributes pcommon.Map) (string, bool) {
	return pdatautil.GetAttributeValue(semconv.AttributeServiceName, attributes)
}

func getFirstMatchingValue(keys []string, attributes ...pcommon.Map) (string, bool) {
	for _, key := range keys {
		for _, attr := range attributes {
			if v, ok := pdatautil.GetAttributeValue(key, attr); ok {
				return v, true
			}
		}
	}
	return "", false
}
