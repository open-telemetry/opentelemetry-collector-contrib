// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/ecs/internal/encoder/log"

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/encoder"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

type defaultEncoder struct {
	dedot bool
	mode  mapping.Mode
}

func (e *defaultEncoder) EncodeLog(resource pcommon.Resource, resourceSchemaURL string, record plog.LogRecord, scope pcommon.InstrumentationScope, scopeSchemaURL string) ([]byte, error) {
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
	encoder.EncodeAttributes(e.mode, &document, record.Attributes())
	document.AddAttributes("Resource", resource.Attributes())
	document.AddAttributes("Scope", encoder.ScopeToAttributes(scope))

	document.Dedup(true)
	var buf bytes.Buffer
	err := document.Serialize(&buf, e.dedot, false)
	return buf.Bytes(), err
}
