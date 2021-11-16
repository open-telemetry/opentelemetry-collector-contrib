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

package datadogreceiver

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strings"

	datadogpb "github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/model/semconv/v1.6.1"
)

func toTraces(traces datadogpb.Traces, req *http.Request) pdata.Traces {
	dest := pdata.NewTraces()
	resSpans := dest.ResourceSpans().AppendEmpty()
	ils := resSpans.InstrumentationLibrarySpans().AppendEmpty()

	ils.InstrumentationLibrary().SetName("Datadog")
	ils.InstrumentationLibrary().SetVersion(req.Header.Get("Datadog-Meta-Tracer-Version"))

	for _, trace := range traces {
		for _, span := range trace {
			newSpan := ils.Spans().AppendEmpty() // TODO: Might be more efficient to resize spans and then populate it

			newSpan.SetTraceID(uInt64ToTraceID(0, span.TraceID))
			newSpan.SetSpanID(uInt64ToSpanID(span.SpanID))
			newSpan.SetStartTimestamp(pdata.Timestamp(span.Start))
			newSpan.SetEndTimestamp(pdata.Timestamp(span.Start + span.Duration))
			newSpan.SetParentSpanID(uInt64ToSpanID(span.ParentID))
			newSpan.SetName(span.Name)
			newSpan.Attributes().InsertString(semconv.AttributeServiceName, span.Service)
			newSpan.Status().SetCode(pdata.StatusCodeOk)

			for k, v := range span.GetMeta() {
				newSpan.Attributes().InsertString(k, v)
			}
			if span.Error > 0 {
				_, errorExists := newSpan.Attributes().Get("error")
				if !errorExists {
					newSpan.Status().SetCode(pdata.StatusCodeError)
				}
			}
			switch span.Type {
			case "web":
				newSpan.SetKind(pdata.SpanKindServer)
			case "client":
				newSpan.SetKind(pdata.SpanKindClient)
			default:
				newSpan.SetKind(pdata.SpanKindUnspecified)
			}

		}
	}

	return dest
}

func decodeRequest(req *http.Request, dest *datadogpb.Traces) error {
	switch mediaType := getMediaType(req); mediaType {
	case "application/msgpack":
		if strings.HasPrefix(req.URL.Path, "/v0.5") {
			reader := datadogpb.NewMsgpReader(req.Body)
			defer datadogpb.FreeMsgpReader(reader)
			return dest.DecodeMsgDictionary(reader)
		} else {
			return msgp.Decode(req.Body, dest)
		}
	case "application/json":
		fallthrough
	case "text/json":
		fallthrough
	case "":
		return json.NewDecoder(req.Body).Decode(dest)
	default:
		// do our best
		if err1 := json.NewDecoder(req.Body).Decode(dest); err1 != nil {
			if err2 := msgp.Decode(req.Body, dest); err2 != nil {
				reader := datadogpb.NewMsgpReader(req.Body)
				defer datadogpb.FreeMsgpReader(reader)
				if err3 := dest.DecodeMsgDictionary(reader); err3 != nil {
					return fmt.Errorf("could not decode JSON (%q), nor Msgpack (%q), nor v0.5 (%q)", err1, err2, err3)
				}
			}
		}
		return nil
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
