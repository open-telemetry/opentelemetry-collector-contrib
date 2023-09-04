// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"bytes"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/objmodel"
)

type mappingModel interface {
	encodeLog(pcommon.Resource, plog.LogRecord) ([]byte, error)
}

// encodeModel tries to keep the event as close to the original open telemetry semantics as is.
// No fields will be mapped by default.
//
// Field deduplication and dedotting of attributes is supported by the encodeModel.
//
// See: https://github.com/open-telemetry/oteps/blob/master/text/logs/0097-log-data-model.md
type encodeModel struct {
	dedup             bool
	dedot             bool
	flattenAttributes bool
	timestampField    string
	unixTime          bool
}

func (m *encodeModel) encodeLog(resource pcommon.Resource, record plog.LogRecord) ([]byte, error) {
	var document objmodel.Document
	if m.flattenAttributes {
		document = objmodel.DocumentFromAttributes(resource.Attributes())
	} else {
		document.AddAttributes("Attributes", resource.Attributes())
	}
	timestampField := "@timestamp"

	if m.timestampField != "" {
		timestampField = m.timestampField
	}

	if m.unixTime {
		document.AddInt(timestampField, epochMilliTimestamp(record))
	} else {
		document.AddTimestamp(timestampField, record.Timestamp())
	}
	document.AddTraceID("TraceId", record.TraceID())
	document.AddSpanID("SpanId", record.SpanID())
	document.AddInt("TraceFlags", int64(record.Flags()))
	document.AddString("SeverityText", record.SeverityText())
	document.AddInt("SeverityNumber", int64(record.SeverityNumber()))
	document.AddAttribute("Body", record.Body())
	if m.flattenAttributes {
		document.AddAttributes("", record.Attributes())
	} else {
		document.AddAttributes("Attributes", record.Attributes())
	}

	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}

func epochMilliTimestamp(record plog.LogRecord) int64 {
	return record.Timestamp().AsTime().UnixMilli()
}
