// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package servicegraphconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

const serviceNameAttr = "service.name"

func findServiceName(attributes pcommon.Map) (string, bool) {
	return pdatautil.GetAttributeValue(serviceNameAttr, attributes)
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
