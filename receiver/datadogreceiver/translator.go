package datadogreceiver

import (
	"encoding/json"
	"fmt"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/consumer/pdata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"mime"
	"net/http"
	"strings"
)

func ToTraces(traces pb.Traces, req *http.Request) pdata.Traces {
	dest := pdata.NewTraces()
	ils := dest.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty()

	// TODO: Pull from HTTP headers ils.InstrumentationLibrary().SetName()
	// TODO: Pull from HTTP headers ils.InstrumentationLibrary().SetVersion()
	for _, trace := range traces {
		for _, span := range trace {
			newSpan := ils.Spans().AppendEmpty() // TODO: Might be more efficient to resize spans and then populate it
			newSpan.SetTraceID(tracetranslator.UInt64ToTraceID(span.TraceID, span.TraceID))
			newSpan.SetSpanID(tracetranslator.UInt64ToSpanID(span.SpanID))
			newSpan.SetStartTimestamp(pdata.Timestamp(span.Start))
			newSpan.SetEndTimestamp(pdata.Timestamp(span.Start + span.Duration))
			newSpan.SetParentSpanID(tracetranslator.UInt64ToSpanID(span.ParentID))
			newSpan.SetName(span.Name)
			for k, v := range span.GetMeta() {
				newSpan.Attributes().InsertString(k, v)
			}

			switch span.Type {
			case "web":
				newSpan.SetKind(pdata.SpanKindSERVER)
			case "client":
				newSpan.SetKind(pdata.SpanKindCLIENT)
			default:
				newSpan.SetKind(pdata.SpanKindUNSPECIFIED)
			}

		}
	}
	return dest
}

func decodeRequest(req *http.Request, dest *pb.Traces) error {
	switch mediaType := getMediaType(req); mediaType {
	case "application/msgpack":
		if strings.Contains(req.RequestURI, "v0.5") {
			reader := pb.NewMsgpReader(req.Body)
			defer pb.FreeMsgpReader(reader)
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
				reader := pb.NewMsgpReader(req.Body)
				defer pb.FreeMsgpReader(reader)
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
