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

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"encoding/binary"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	datadogpb "github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.6.1"
)

func toTraces(traces datadogpb.Traces, req *http.Request) ptrace.Traces {
	dest := ptrace.NewTraces()
	resSpans := dest.ResourceSpans().AppendEmpty()
	resSpans.SetSchemaUrl(semconv.SchemaURL)

	for _, trace := range traces {
		ils := resSpans.ScopeSpans().AppendEmpty()
		ils.Scope().SetName("Datadog-" + req.Header.Get("Datadog-Meta-Lang"))
		ils.Scope().SetVersion(req.Header.Get("Datadog-Meta-Tracer-Version"))
		spans := ptrace.NewSpanSlice()
		spans.EnsureCapacity(len(trace))
		for _, span := range trace {
			newSpan := spans.AppendEmpty()

			newSpan.SetTraceID(uInt64ToTraceID(0, span.TraceID))
			newSpan.SetSpanID(uInt64ToSpanID(span.SpanID))
			newSpan.SetStartTimestamp(pcommon.Timestamp(span.Start))
			newSpan.SetEndTimestamp(pcommon.Timestamp(span.Start + span.Duration))
			newSpan.SetParentSpanID(uInt64ToSpanID(span.ParentID))
			newSpan.SetName(span.Resource)

			if span.Error > 0 {
				newSpan.Status().SetCode(ptrace.StatusCodeError)
			} else {
				newSpan.Status().SetCode(ptrace.StatusCodeOk)
			}

			attrs := newSpan.Attributes()
			attrs.EnsureCapacity(len(span.GetMeta()) + 1)
			attrs.InsertString(semconv.AttributeServiceName, span.Service)
			for k, v := range span.GetMeta() {
				k = translateDataDogKeyToOtel(k)
				if len(k) > 0 {
					attrs.InsertString(k, v)
				}
			}

			switch span.Type {
			case "web":
				newSpan.SetKind(ptrace.SpanKindServer)
			case "custom":
				newSpan.SetKind(ptrace.SpanKindUnspecified)
			default:
				newSpan.SetKind(ptrace.SpanKindClient)
			}
		}
		spans.MoveAndAppendTo(ils.Spans())
	}

	return dest
}

func translateDataDogKeyToOtel(k string) string {
	// Tags prefixed with _dd. are for Datadog's use only, adding them to another backend will just increase cardinality needlessly
	if strings.HasPrefix(k, "_dd.") {
		return ""
	}
	switch strings.ToLower(k) {
	case "env":
		return semconv.AttributeDeploymentEnvironment
	case "version":
		return semconv.AttributeServiceVersion
	case "container_id":
		return semconv.AttributeContainerID
	case "container_name":
		return semconv.AttributeContainerName
	case "image_name":
		return semconv.AttributeContainerImageName
	case "image_tag":
		return semconv.AttributeContainerImageTag
	case "process_id":
		return semconv.AttributeProcessPID
	case "error.stacktrace":
		return semconv.AttributeExceptionStacktrace
	case "error.msg":
		return semconv.AttributeExceptionMessage
	default:
		return k
	}

}

func decodeRequest(req *http.Request, dest *datadogpb.Traces) error {
	switch mediaType := getMediaType(req); mediaType {
	case "application/msgpack":
		if strings.HasPrefix(req.URL.Path, "/v0.5") {
			reader := datadogpb.NewMsgpReader(req.Body)
			defer datadogpb.FreeMsgpReader(reader)
			return dest.DecodeMsgDictionary(reader)
		}
		return msgp.Decode(req.Body, dest)
	default:
		return json.NewDecoder(req.Body).Decode(dest)
	}
}

func getMediaType(req *http.Request) string {
	mt, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return "application/json"
	}
	return mt
}

func uInt64ToTraceID(high, low uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[:8], high)
	binary.BigEndian.PutUint64(traceID[8:], low)
	return pcommon.NewTraceID(traceID)
}

func uInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pcommon.NewSpanID(spanID)
}
