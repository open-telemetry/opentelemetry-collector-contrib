// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mapping // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

// DefaultEncoder is an encoder that handles the `none` and `raw` encoding modes.
type DefaultEncoder struct {
	Mode
}

func (e DefaultEncoder) EncodeLog(resource pcommon.Resource, record plog.LogRecord, scopeLogs plog.ScopeLogs) objmodel.Document {
	var document objmodel.Document

	docTimeStamp := record.Timestamp()
	if docTimeStamp.AsTime().UnixNano() == 0 {
		docTimeStamp = record.ObservedTimestamp()
	}
	document.AddTimestamp("@timestamp", docTimeStamp) // We use @timestamp in order to ensure that we can index if the default data stream logs template is used.
	document.AddTraceID("TraceId", record.TraceID())
	document.AddSpanID("SpanId", record.SpanID())
	document.AddInt("TraceFlags", int64(record.Flags()))
	document.AddString("SeverityText", record.SeverityText())
	document.AddInt("SeverityNumber", int64(record.SeverityNumber()))
	document.AddAttribute("Body", record.Body())
	encodeAttributes(&document, e.Mode, record.Attributes())
	document.AddAttributes("Resource", resource.Attributes())
	document.AddAttributes("Scope", scopeToAttributes(scopeLogs.Scope()))

	return document
}

func encodeAttributes(document *objmodel.Document, m Mode, attributes pcommon.Map) {
	key := "Attributes"
	if m == ModeRaw {
		key = ""
	}
	document.AddAttributes(key, attributes)
}

func scopeToAttributes(scope pcommon.InstrumentationScope) pcommon.Map {
	attrs := pcommon.NewMap()
	attrs.PutStr("name", scope.Name())
	attrs.PutStr("version", scope.Version())
	for k, v := range scope.Attributes().AsRaw() {
		attrs.PutStr(k, v.(string))
	}
	return attrs
}
