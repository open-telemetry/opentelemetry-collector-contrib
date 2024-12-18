// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoder // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/encoder"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

func ScopeToAttributes(scope pcommon.InstrumentationScope) pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("name", scope.Name())
	attrs.PutStr("version", scope.Version())
	for k, v := range scope.Attributes().AsRaw() {
		attrs.PutStr(k, v.(string))
	}
	return attrs
}

func EncodeAttributes(mode mapping.Mode, document *objmodel.Document, attributes pcommon.Map) {
	key := "Attributes"
	if mode == mapping.ModeRaw {
		key = ""
	}
	document.AddAttributes(key, attributes)
}
