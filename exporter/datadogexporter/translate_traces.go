// Copyright The OpenTelemetry Authors
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

package datadogexporter

import (
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/internaldata"
	"encoding/hex"
	"net/http"
	"strconv"
	"fmt"

	apm "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/apm"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	octrace "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	"go.opencensus.io/trace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
)

// codeDetails specifies information about a trace status code.
type codeDetails struct {
	message string // status message
	status  int    // corresponding HTTP status code
}

// ddSpan represents the Datadog span definition.
type ddSpan struct {
	SpanID   uint64             `msg:"span_id"`
	TraceID  uint64             `msg:"trace_id"`
	ParentID uint64             `msg:"parent_id"`
	Name     string             `msg:"name"`
	Service  string             `msg:"service"`
	Resource string             `msg:"resource"`
	Type     string             `msg:"type"`
	Start    int64              `msg:"start"`
	Duration int64              `msg:"duration"`
	Meta     map[string]string  `msg:"meta,omitempty"`
	Metrics  map[string]float64 `msg:"metrics,omitempty"`
	Error    int32              `msg:"error"`
}

const (
	keySamplingPriority  = "_sampling_priority_v1"
	keyStatusDescription = "opencensus.status_description"
	keyStatusCode        = "opencensus.status_code"
	keyStatus            = "opencensus.status"
	keySpanName          = "span.name"
	// keySamplingPriorityRate = "_sampling_priority_rate_v1"
	instrumentationLibraryName    string = "instrumentation_library.name"
)

// statusCodes maps (*trace.SpanData).Status.Code to their message and http status code. See:
// https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto.
var statusCodes = map[int32]codeDetails{
	trace.StatusCodeOK:                 {message: "OK", status: http.StatusOK},
	trace.StatusCodeCancelled:          {message: "CANCELLED", status: 499},
	trace.StatusCodeUnknown:            {message: "UNKNOWN", status: http.StatusInternalServerError},
	trace.StatusCodeInvalidArgument:    {message: "INVALID_ARGUMENT", status: http.StatusBadRequest},
	trace.StatusCodeDeadlineExceeded:   {message: "DEADLINE_EXCEEDED", status: http.StatusGatewayTimeout},
	trace.StatusCodeNotFound:           {message: "NOT_FOUND", status: http.StatusNotFound},
	trace.StatusCodeAlreadyExists:      {message: "ALREADY_EXISTS", status: http.StatusConflict},
	trace.StatusCodePermissionDenied:   {message: "PERMISSION_DENIED", status: http.StatusForbidden},
	trace.StatusCodeResourceExhausted:  {message: "RESOURCE_EXHAUSTED", status: http.StatusTooManyRequests},
	trace.StatusCodeFailedPrecondition: {message: "FAILED_PRECONDITION", status: http.StatusBadRequest},
	trace.StatusCodeAborted:            {message: "ABORTED", status: http.StatusConflict},
	trace.StatusCodeOutOfRange:         {message: "OUT_OF_RANGE", status: http.StatusBadRequest},
	trace.StatusCodeUnimplemented:      {message: "UNIMPLEMENTED", status: http.StatusNotImplemented},
	trace.StatusCodeInternal:           {message: "INTERNAL", status: http.StatusInternalServerError},
	trace.StatusCodeUnavailable:        {message: "UNAVAILABLE", status: http.StatusServiceUnavailable},
	trace.StatusCodeDataLoss:           {message: "DATA_LOSS", status: http.StatusNotImplemented},
	trace.StatusCodeUnauthenticated:    {message: "UNAUTHENTICATED", status: http.StatusUnauthorized},
}

func convertToDatadogTd(td pdata.Traces, cfg *Config) []*pb.TracePayload {
	overrideHostname := cfg.Hostname != ""

	// TODO: move this over to using Otel internals directly rather than relying on OpenCensus
	octds := internaldata.TraceDataToOC(td)

	traces := []*pb.TracePayload{}

	for _, octd := range octds {
		// surely there must be a cleaner way
		// This appears to not pull in a hostname?
		var hostname string

		if overrideHostname {
			hostname = *GetHost(cfg)
		} else {
			hostname = extractHostName(octd.Node)
		}


		// fmt.Printf("HOSTNAME IS")
		// fmt.Printf(hostname)
		// fmt.Printf("get host returns")
		// fmt.Printf(*GetHost(cfg))
		// fmt.Printf("extractHostName returns")
		// fmt.Printf(extractHostName(octd.Node))

		apiTraces := map[uint64]*pb.APITrace{}

		for _, span := range octd.Spans {

			pbSpan := convertSpan(span)

			var apiTrace *pb.APITrace
			var ok bool

			if apiTrace, ok = apiTraces[pbSpan.TraceID]; !ok {
				apiTrace = &pb.APITrace{
					TraceID:   pbSpan.TraceID,
					Spans:     []*pb.Span{},
					StartTime: 0,
					EndTime:   0,
				}
				apiTraces[apiTrace.TraceID] = apiTrace
			}
			
			addToAPITrace(apiTrace, pbSpan)
		}


		// TODO: Use Env Config here, default to 'none'
		payload := pb.TracePayload{
			HostName:     hostname,
			Env:          "oteltest",
			Traces:       []*pb.APITrace{},
			Transactions: []*pb.Span{},
		}

		for _, apiTrace := range apiTraces {
			top := apm.GetAnalyzedSpans(apiTrace.Spans)
			apm.ComputeSublayerMetrics(apiTrace.Spans)
			payload.Transactions = append(payload.Transactions, top...)
			payload.Traces = append(payload.Traces, apiTrace)
		}		

		traces = append(traces, &payload)

	}

	return traces
}

// convertSpan takes an OpenCensus span and returns a Datadog span.
func convertSpan(s *octrace.Span) *pb.Span {
	startTime := pdata.TimestampToUnixNano(s.StartTime)
	endTime := pdata.TimestampToUnixNano(s.EndTime)

	// TODOs:
	// span.Type
	// Have span.Resource give better info for http spans

	span := &pb.Span{
		TraceID:  decodeAPMId(hex.EncodeToString(s.TraceId[:])),
		SpanID:  decodeAPMId(hex.EncodeToString(s.SpanId[:])),
		Name:     getSpanName(s),
		Resource: s.Name.Value,
		Service:  "service_example",
		Start:    int64(startTime),
		Duration: int64(endTime - startTime),
		Metrics:  map[string]float64{},
		Meta:     map[string]string{},
	}

	// TODO: confirm parentId approach
	if len(s.ParentSpanId) > 0 {
		idVal := decodeAPMId(hex.EncodeToString(s.ParentSpanId[:]))

		span.ParentID = idVal
	} else {
		span.ParentID = 0
	}

	// TODO: figure out error code handling 
	// fmt.Printf("%v", s.Status.Code)
	// code, ok := statusCodes[s.Status.Code]
	// isErr := s.Status == nil

	// if !isErr {

	// 	code, ok := statusCodes[trace.StatusCodeOK]
	// 	if !ok {
	// 		code = codeDetails{
	// 			message: "ERR_CODE_" + strconv.FormatInt(int64(s.Status.Code), 10),
	// 			status:  http.StatusInternalServerError,
	// 		}
	// 	}

	// 	switch s.Kind {
	// 	case trace.SpanKindClient:
	// 		span.Type = "client"
	// 		if code.status/100 == 4 {
	// 			span.Error = 1
	// 		}
	// 	case trace.SpanKindServer:
	// 		span.Type = "server"
	// 		fallthrough
	// 	default:
	// 		span.Type = "custom"
	// 		if code.status/100 == 5 {
	// 			span.Error = 1
	// 		}
	// 	}

	// 	if span.Error == 1 {
	// 		span.Meta[ext.ErrorType] = code.message
	// 		if msg := s.Status.Message; msg != "" {
	// 			span.Meta[ext.ErrorMsg] = msg
	// 		}
	// 	}

	// 	span.Meta[keyStatusCode] = strconv.Itoa(int(s.Status.Code))
	// 	span.Meta[keyStatus] = code.message
	// 	if msg := s.Status.Message; msg != "" {
	// 		span.Meta[keyStatusDescription] = msg
	// 	}
	// } else {
	// 	span.Type = ""
	// }
	span.Type = ""

	// TODO: Apply Config Tags
	// for key, val := range e.opts.GlobalTags {
	// 	setTag(span, key, val)
	// }

	// Set Attributes as Tags
	for key, val := range s.GetAttributes().GetAttributeMap() {
		setTag(span, key, attributeValueToString(val))
	}
	return span
}

func setTag(s *pb.Span, key string, val interface{}) {
	if key == ext.Error {
		setError(s, val)
		return
	}
	switch v := val.(type) {
	case string:
		setStringTag(s, key, v)
	case bool:
		if v {
			setStringTag(s, key, "true")
		} else {
			setStringTag(s, key, "false")
		}
	case float64:
		setMetric(s, key, v)
	case int64:
		setMetric(s, key, float64(v))
	default:
		// should never happen according to docs, nevertheless
		// we should account for this to avoid exceptions
		setStringTag(s, key, fmt.Sprintf("%v", v))
	}
}

func setMetric(s *pb.Span, key string, v float64) {
	switch key {
	case ext.SamplingPriority:
		s.Metrics[keySamplingPriority] = v
	default:
		s.Metrics[key] = v
	}
}

func setStringTag(s *pb.Span, key, v string) {
	switch key {
	case ext.ServiceName:
		s.Service = v
	case ext.ResourceName:
		s.Resource = v
	case ext.SpanType:
		s.Type = v
	case ext.AnalyticsEvent:
		if v != "false" {
			setMetric(s, ext.EventSampleRate, 1)
		} else {
			setMetric(s, ext.EventSampleRate, 0)
		}
	case keySpanName:
		s.Name = v
	default:
		s.Meta[key] = v
	}
}

func setError(s *pb.Span, val interface{}) {
	switch v := val.(type) {
	case string:
		s.Error = 1
		s.Meta[ext.ErrorMsg] = v
	case bool:
		if v {
			s.Error = 1
		} else {
			s.Error = 0
		}
	case int64:
		if v > 0 {
			s.Error = 1
		} else {
			s.Error = 0
		}
	case nil:
		s.Error = 0
	default:
		s.Error = 1
	}
}

func addToAPITrace(apiTrace *pb.APITrace, sp *pb.Span) {
	apiTrace.Spans = append(apiTrace.Spans, sp)
	endTime := sp.Start + sp.Duration
	if apiTrace.EndTime > endTime {
		apiTrace.EndTime = endTime
	}
	if apiTrace.StartTime == 0 || apiTrace.StartTime > sp.Start {
		apiTrace.StartTime = sp.Start
	}
}

func decodeAPMId(id string) uint64 {
	if len(id) > 16 {
		id = id[len(id)-16:]
	}
	val, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		return 0
	}
	return val
}

func attributeValueToString(v *tracepb.AttributeValue) string {
	switch attribValue := v.Value.(type) {
	case *tracepb.AttributeValue_StringValue:
		return truncableStringToStr(attribValue.StringValue)
	case *tracepb.AttributeValue_IntValue:
		return strconv.FormatInt(attribValue.IntValue, 10)
	case *tracepb.AttributeValue_BoolValue:
		if attribValue.BoolValue {
			return "true"
		}
		return "false"
	case *tracepb.AttributeValue_DoubleValue:
		return strconv.FormatFloat(attribValue.DoubleValue, 'g', -1, 64)
	default:
	}
	return "<Unknown OpenCensus Attribute>"
}

// func spanKindToStr(spanKind tracepb.Span_SpanKind) string {
// 	switch spanKind {
// 	case tracepb.Span_CLIENT:
// 		return string(tracetranslator.OpenTracingSpanKindClient)
// 	case tracepb.Span_SERVER:
// 		return string(tracetranslator.OpenTracingSpanKindServer)
// 	}
// 	return ""
// }

func truncableStringToStr(ts *tracepb.TruncatableString) string {
	if ts == nil {
		return ""
	}
	return ts.Value
}

func extractHostName(node *commonpb.Node) string {
	if process := node.GetIdentifier(); process != nil {
		if len(process.HostName) != 0 {
			return process.HostName
		}
	}
	return ""
}

// TODO:
// test this
func getSpanName(s *octrace.Span) string {
	// largely a port of logic here 
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L213
	// Get span name by using instrumentation and kind while backing off to span.kind	
	instrumentationlibrary := ""
	for key, val := range s.GetAttributes().GetAttributeMap() {
		if key == instrumentationLibraryName {
			instrumentationlibrary = attributeValueToString(val)
		}
	}

	if instrumentationlibrary != "" {
		return fmt.Sprintf("%s.%s", instrumentationlibrary, s.Kind)
	} else {
		return fmt.Sprintf("%s.%s", "opentelemetry", s.Kind)
	}
}
