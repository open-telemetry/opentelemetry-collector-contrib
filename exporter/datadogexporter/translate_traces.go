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
	"go.opentelemetry.io/collector/translator/conventions"
	// "go.opentelemetry.io/collector/translator/internaldata"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"encoding/hex"
	"net/http"
	"strconv"
	"fmt"

	apm "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/apm"
	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	// tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
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
	instrumentationLibraryName string = "otel.instrumentation_library.name"
	httpStatusCode string = "http.status_code"
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

func convertToDatadogTd(td pdata.Traces, cfg *Config) ([]*pb.TracePayload, error) {
	// this is getting abstracted out to config
	// overrideHostname := cfg.Hostname != ""
	hostname := *GetHost(cfg)

	resourceSpans := td.ResourceSpans()

	traces := []*pb.TracePayload{}

	if resourceSpans.Len() == 0 {
		return nil, nil
	}

	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		if rs.IsNil() {
			continue
		}

		payload, err := resourceSpansToDatadogSpans(rs, hostname)
		if err != nil {
			return traces, err
		}
		
		traces = append(traces, &payload)
		
	}

	return traces, nil
}

func resourceSpansToDatadogSpans(rs pdata.ResourceSpans, hostname string) (pb.TracePayload, error) {
	resource := rs.Resource()
	ilss := rs.InstrumentationLibrarySpans()

	// TODO: Use Env Config here, default to 'none'
	payload := pb.TracePayload{
		HostName:     hostname,
		Env:          "oteltest",
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	if resource.IsNil() && ilss.Len() == 0 {
		return payload, nil
	}

	localServiceName, datadogTags := resourceToDatadogServiceNameAndAttributeMap(resource)

	apiTraces := map[uint64]*pb.APITrace{}

	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		if ils.IsNil() {
			continue
		}
		extractInstrumentationLibraryTags(ils.InstrumentationLibrary(), datadogTags)
		spans := ils.Spans()
		for j := 0; j < spans.Len(); j++ {
			span, err := spanToDatadogSpan(spans.At(j), localServiceName, datadogTags)

			if err != nil {
				return payload, err
			} else {
				var apiTrace *pb.APITrace
				var ok bool

				if apiTrace, ok = apiTraces[span.TraceID]; !ok {
					apiTrace = &pb.APITrace{
						TraceID:   span.TraceID,
						Spans:     []*pb.Span{},
						StartTime: 0,
						EndTime:   0,
					}
					apiTraces[apiTrace.TraceID] = apiTrace
				}
				
				addToAPITrace(apiTrace, span)
			}
		}
	}

	for _, apiTrace := range apiTraces {
		top := apm.GetAnalyzedSpans(apiTrace.Spans)
		apm.ComputeSublayerMetrics(apiTrace.Spans)
		payload.Transactions = append(payload.Transactions, top...)
		payload.Traces = append(payload.Traces, apiTrace)
	}

	return payload, nil
}

func resourceToDatadogServiceNameAndAttributeMap(
	resource pdata.Resource,
) (serviceName string, datadogTags map[string]string) {

	datadogTags = make(map[string]string)
	if resource.IsNil() {
		return tracetranslator.ResourceNotSet, datadogTags
	}

	attrs := resource.Attributes()
	if attrs.Len() == 0 {
		return tracetranslator.ResourceNoAttrs, datadogTags
	}

	attrs.ForEach(func(k string, v pdata.AttributeValue) {
		datadogTags[k] = tracetranslator.AttributeValueToString(v, false)
	})

	serviceName = extractDatadogServiceName(datadogTags)
	return serviceName, datadogTags
}

func extractDatadogServiceName(datadogTags map[string]string) string {
	var serviceName string
	if sn, ok := datadogTags[conventions.AttributeServiceName]; ok {
		serviceName = sn
		delete(datadogTags, conventions.AttributeServiceName)
	} else {
		serviceName = tracetranslator.ResourceNoServiceName
	}
	return serviceName
}

func extractInstrumentationLibraryTags(il pdata.InstrumentationLibrary, datadogTags map[string]string) {
	if il.IsNil() {
		return
	}
	if ilName := il.Name(); ilName != "" {
		datadogTags[tracetranslator.TagInstrumentationName] = ilName
	}
	if ilVer := il.Version(); ilVer != "" {
		datadogTags[tracetranslator.TagInstrumentationVersion] = ilVer
	}
}

// convertSpan takes an OpenCensus span and returns a Datadog span.
func spanToDatadogSpan(	s pdata.Span,
	localServiceName string,
	datadogTags map[string]string,
) (*pb.Span, error) {
	tags := aggregateSpanTags(s, datadogTags)

	// get tracestate as just a general tag
	if len(s.TraceState()) > 0 {
		tags[tracetranslator.TagW3CTraceState] = string(s.TraceState())
	}
	startTime := s.StartTime()
	endTime := s.EndTime()
	duration := int64(endTime) - int64(startTime)

	if s.EndTime() == 0 {
		duration = int64(0)
	}

	datadogType := spanKindToDatadogType(s.Kind())


	// TODOs:
	// span.Type
	// Have span.Resource give better info for http spans

	span := &pb.Span{
		TraceID:  decodeAPMId(hex.EncodeToString(s.TraceID().Bytes()[:])),
		SpanID:  decodeAPMId(hex.EncodeToString(s.SpanID().Bytes()[:])),
		Name:     getSpanName(s, tags),
		Resource: s.Name(),
		Service:  "service_example",
		Start:    int64(startTime),
		Duration: int64(duration),
		Metrics:  map[string]float64{},
		Meta:     map[string]string{},
		Type: datadogType,
	}

	// TODO: confirm parentId approach
	if len(s.ParentSpanID().Bytes()) > 0 {
		idVal := decodeAPMId(hex.EncodeToString(s.ParentSpanID().Bytes()[:]))

		span.ParentID = idVal
	} else {
		span.ParentID = 0
	}

	// Set Span Status and any response or error details
	status := s.Status()
	if !status.IsNil() {
		// map to known msg + status
		code, ok := statusCodes[int32(status.Code())]
		if !ok {
			code = codeDetails{
				message: "ERR_CODE_" + strconv.FormatInt(int64(status.Code()), 10),
				status:  http.StatusInternalServerError,
			}
		}

		// check if 500 or 400 level, if so set error
		if code.status/100 == 5 || code.status/100 == 4 {
			span.Error = 1
			// set error type
			tags[ext.ErrorType] = code.message
			// set error message
			if status.Message() != "" {
				tags[ext.ErrorMsg] = status.Message()
			} else {
				tags[ext.ErrorMsg] = code.message
			}
		// otherwise no error
		} else {
			span.Error = 0
		}

		// set tag as string for status code
		tags[httpStatusCode] = status.Code().String()
	} else {
		span.Error = 0
	}


	// TODO: Apply Config Tags
	// for key, val := range e.opts.GlobalTags {
	// 	setTag(span, key, val)
	// }

	// Set Attributes as Tags
	for key, val := range tags {
		setTag(span, key, val)
	}

	return span, nil
}

func aggregateSpanTags(span pdata.Span, datadogTags map[string]string) map[string]string {
	tags := make(map[string]string)
	for key, val := range datadogTags {
		tags[key] = val
	}
	spanTags := attributeMapToStringMap(span.Attributes())
	for key, val := range spanTags {
		tags[key] = val
	}
	return tags
}

func attributeMapToStringMap(attrMap pdata.AttributeMap) map[string]string {
	rawMap := make(map[string]string)
	attrMap.ForEach(func(k string, v pdata.AttributeValue) {
		rawMap[k] = tracetranslator.AttributeValueToString(v, false)
	})
	return rawMap
}

func spanKindToDatadogType(kind pdata.SpanKind) string {
	switch kind {
	case pdata.SpanKindCLIENT:
		return "client"
	case pdata.SpanKindSERVER:
		return "server"
	default:
		return "custom"
	}
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
func getSpanName(s pdata.Span, datadogTags map[string]string) string {
	// largely a port of logic here 
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L213
	// Get span name by using instrumentation and kind while backing off to span.kind	
	instrumentationlibrary := ""
	for key, val := range datadogTags {
		if key == instrumentationLibraryName {
			instrumentationlibrary = val
		}
	}

	if instrumentationlibrary != "" {
		return fmt.Sprintf("%s.%s", instrumentationlibrary, s.Kind())
	} else {
		return fmt.Sprintf("%s.%s", "opentelemetry", s.Kind())
	}
}
