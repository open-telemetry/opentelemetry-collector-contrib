// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const tsLayout = "2006-01-02T15:04:05.000000000Z"

func isGeoAttribute(k string, val pcommon.Value) bool {
	if val.Type() != pcommon.ValueTypeDouble {
		return false
	}
	switch k {
	case "geo.location.lat", "geo.location.lon":
		return true
	}
	return strings.HasSuffix(k, ".geo.location.lat") || strings.HasSuffix(k, ".geo.location.lon")
}
