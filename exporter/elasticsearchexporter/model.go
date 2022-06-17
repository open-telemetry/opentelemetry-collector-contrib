// Copyright 2021, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"encoding/json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

type mappingModel interface {
	encodeLog(pcommon.Resource, plog.LogRecord) ([]byte, error)
	encodeSpan(pcommon.Resource, ptrace.Span) ([]byte, error)
}

// encodeModel tries to keep the event as close to the original open telemetry semantics as is.
// No fields will be mapped by default.
//
// Field deduplication and dedotting of attributes is supported by the encodeModel.
//
// See: https://github.com/open-telemetry/oteps/blob/master/text/logs/0097-log-data-model.md
type encodeModel struct {
	dedup bool
	dedot bool
}

func (m *encodeModel) encodeLog(resource pcommon.Resource, record plog.LogRecord) ([]byte, error) {
	var document objmodel.Document
	document.AddTimestamp("@timestamp", record.Timestamp()) // We use @timestamp in order to ensure that we can index if the default data stream logs template is used.
	document.AddID("TraceId", record.TraceID())
	document.AddID("SpanId", record.SpanID())
	document.AddInt("TraceFlags", int64(record.Flags()))
	document.AddString("SeverityText", record.SeverityText())
	document.AddInt("SeverityNumber", int64(record.SeverityNumber()))
	document.AddAttribute("Body", record.Body())
	document.AddAttributes("Attributes", record.Attributes())
	document.AddAttributes("Resource", resource.Attributes())

	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}

func (m *encodeModel) encodeSpan(resource pcommon.Resource, span ptrace.Span) ([]byte, error) {
	var document objmodel.Document
	document.AddTimestamp("@timestamp", span.StartTimestamp()) // We use @timestamp in order to ensure that we can index if the default data stream logs template is used.
	document.AddTimestamp("endTimestamp", span.EndTimestamp()) // We use @timestamp in order to ensure that we can index if the default data stream logs template is used.
	document.AddID("TraceId", span.TraceID())
	document.AddID("SpanId", span.SpanID())
	document.AddID("ParentSpanId", span.ParentSpanID())
	document.AddString("Name", span.Name())
	document.AddString("Kind", span.Kind().String())
	document.AddInt("TraceStatus", int64(span.Status().Code()))
	linkJson, _ := json.Marshal(span.Links())
	document.AddString("Link", string(linkJson))
	document.AddAttributes("Attributes", span.Attributes())
	document.AddAttributes("Resource", resource.Attributes())

	if m.dedup {
		document.Dedup()
	} else if m.dedot {
		document.Sort()
	}

	var buf bytes.Buffer
	err := document.Serialize(&buf, m.dedot)
	return buf.Bytes(), err
}
