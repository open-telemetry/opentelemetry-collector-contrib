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
	"go.opentelemetry.io/collector/model/pdata"
	semconv "go.opentelemetry.io/collector/model/semconv/v1.6.1"
)

func toTraces(traces datadogpb.Traces, req *http.Request) pdata.Traces {
	dest := pdata.NewTraces()
	resSpans := dest.ResourceSpans().AppendEmpty()
	resSpans.SetSchemaUrl(semconv.SchemaURL)

	for _, trace := range traces {
		ils := resSpans.InstrumentationLibrarySpans().AppendEmpty()
		ils.InstrumentationLibrary().SetName("Datadog-" + req.Header.Get("Datadog-Meta-Lang"))
		ils.InstrumentationLibrary().SetVersion(req.Header.Get("Datadog-Meta-Tracer-Version"))
		spans := pdata.NewSpanSlice()
		spans.EnsureCapacity(len(trace))
		for _, span := range trace {
			newSpan := spans.AppendEmpty()

			newSpan.SetTraceID(uInt64ToTraceID(0, span.TraceID))
			newSpan.SetSpanID(uInt64ToSpanID(span.SpanID))
			newSpan.SetStartTimestamp(pdata.Timestamp(span.Start))
			newSpan.SetEndTimestamp(pdata.Timestamp(span.Start + span.Duration))
			newSpan.SetParentSpanID(uInt64ToSpanID(span.ParentID))
			newSpan.SetName(span.Name)

			if span.Error > 0 {
				newSpan.Status().SetCode(pdata.StatusCodeError)
			} else {
				newSpan.Status().SetCode(pdata.StatusCodeOk)
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
				newSpan.SetKind(pdata.SpanKindServer)
			case "custom":
				newSpan.SetKind(pdata.SpanKindUnspecified)
			default:
				newSpan.SetKind(pdata.SpanKindClient)
			}
		}
		spans.MoveAndAppendTo(ils.Spans())
	}

	return dest
}

func translateDataDogKeyToOtel(k string) string {
	// We dont want these
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

func uInt64ToTraceID(high, low uint64) pdata.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[:8], high)
	binary.BigEndian.PutUint64(traceID[8:], low)
	return pdata.NewTraceID(traceID)
}

func uInt64ToSpanID(id uint64) pdata.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pdata.NewSpanID(spanID)
}
