// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// buildAttributeRoutingKey encodes a missing attribute as "name=|".
func buildAttributeRoutingKey(attr string) string {
	return attr + "=|"
}

// buildAttributeRoutingKeyStrValue encodes a single string attribute key/value pair as
// "name=value|".
func buildAttributeRoutingKeyStrValue(attr, value string) string {
	return attr + "=" + value + "|"
}

// buildAttributeRoutingKeyValue encodes a single attribute key/value pair as
// "name=value|".
func buildAttributeRoutingKeyValue(attr string, value pcommon.Value) string {
	return attr + "=" + value.AsString() + "|"
}
