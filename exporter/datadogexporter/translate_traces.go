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
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/DataDog/datadog-agent/pkg/trace/pb"
	"go.opencensus.io/trace"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
)

// codeDetails specifies information about a trace status code.
type codeDetails struct {
	message string // status message
	status  int    // corresponding HTTP status code
}

const (
	deploymentEnv              string = "deployment.environment"
	keySamplingPriority        string = "_sampling_priority_v1"
	keySpanName                string = "span.name"
	instrumentationLibraryName string = "otel.instrumentation_library.name"
	httpStatusCode             string = "http.status_code"
	httpMethod                 string = "http.method"
	httpRoute                  string = "http.route"
	versionTag                 string = "version"
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

// converts Traces into an array of datadog trace payloads grouped by env
func convertToDatadogTd(td pdata.Traces, cfg *Config, globalTags []string) ([]*pb.TracePayload, error) {
	// get hostname tag
	// this is getting abstracted out to config
	hostname := *GetHost(cfg)

	// get env tag
	env := cfg.Env
	service := cfg.Service
	version := cfg.Version

	// TODO:
	// do we apply other global tags, like version+service, to every span or only root spans of a service
	// should globalTags['service'] take precedence over a trace's resource.service.name? I don't believe so, need to confirm

	resourceSpans := td.ResourceSpans()

	traces := []*pb.TracePayload{}

	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		if rs.IsNil() {
			continue
		}

		// TODO: Also pass in globalTags here when we know what to do with them
		payload, err := resourceSpansToDatadogSpans(rs, hostname, env, service, version)
		if err != nil {
			return traces, err
		}

		traces = append(traces, &payload)

	}

	return traces, nil
}

// converts a Trace's resource spans into a trace payload
func resourceSpansToDatadogSpans(rs pdata.ResourceSpans, hostname string, env string, service string, version string) (pb.TracePayload, error) {
	resource := rs.Resource()
	ilss := rs.InstrumentationLibrarySpans()

	// TODO: should we be getting env from the trace's spans itself? if so what's the tag+precedence order?
	payload := pb.TracePayload{
		HostName:     hostname,
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	if resource.IsNil() && ilss.Len() == 0 {
		return payload, nil
	}

	localServiceName, datadogTags := resourceToDatadogServiceNameAndAttributeMap(resource)

	// specification states that the resource level deployment.environment should be used for passing env, so defer to that
	// https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/resource/semantic_conventions/deployment_environment.md#deployment
	if resourceEnv, ok := datadogTags[deploymentEnv]; ok {
		payload.Env = resourceEnv
	}

	apiTraces := map[uint64]*pb.APITrace{}

	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		if ils.IsNil() {
			continue
		}
		extractInstrumentationLibraryTags(ils.InstrumentationLibrary(), datadogTags)
		spans := ils.Spans()
		for j := 0; j < spans.Len(); j++ {
			span, err := spanToDatadogSpan(spans.At(j), localServiceName, datadogTags, service, version)

			if err != nil {
				return payload, err
			}

			var apiTrace *pb.APITrace
			var ok bool

			if apiTrace, ok = apiTraces[span.TraceID]; !ok {
				// we default these values to 0 and then calculate the appropriate StartTime
				// and EndTime within addToAPITrace()
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

	for _, apiTrace := range apiTraces {
		top := GetAnalyzedSpans(apiTrace.Spans)
		ComputeSublayerMetrics(apiTrace.Spans)
		payload.Transactions = append(payload.Transactions, top...)
		payload.Traces = append(payload.Traces, apiTrace)
	}

	return payload, nil
}

// convertSpan takes an internal span representation and returns a Datadog span.
func spanToDatadogSpan(s pdata.Span,
	localServiceName string,
	datadogTags map[string]string,
	service string,
	version string,
) (*pb.Span, error) {
	tags := aggregateSpanTags(s, datadogTags)

	// if no version tag exists, set it if provided via config
	if version != "" {
		if tagVersion := tags[versionTag]; tagVersion == "" {
			tags[versionTag] = version
		}
	}

	// get tracestate as just a general tag
	if len(s.TraceState()) > 0 {
		tags[tracetranslator.TagW3CTraceState] = string(s.TraceState())
	}

	// get start/end time to calc duration
	startTime := s.StartTime()
	endTime := s.EndTime()
	duration := int64(endTime) - int64(startTime)

	// it's possible end time is unset, so default to 0 rather than using a negative number
	if s.EndTime() == 0 {
		duration = 0
	}

	datadogType := spanKindToDatadogType(s.Kind())

	span := &pb.Span{
		TraceID:  decodeAPMId(hex.EncodeToString(s.TraceID().Bytes()[:])),
		SpanID:   decodeAPMId(hex.EncodeToString(s.SpanID().Bytes()[:])),
		Name:     getDatadogSpanName(s, tags),
		Resource: getDatadogResourceName(s, tags),
		Service:  service,
		Start:    int64(startTime),
		Duration: int64(duration),
		Metrics:  map[string]float64{},
		Meta:     map[string]string{},
		Type:     datadogType,
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
		}

		// set tag as string for status code
		tags[httpStatusCode] = status.Code().String()
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

// TODO: determine why this always seems to be SPAN_KIND_UNSPECIFIED
// even though span.kind is getting set at the app tracer level
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
	// if a span has `service.name` set as the tag
	case ext.ServiceName:
		s.Service = v
	case ext.SpanType:
		s.Type = v
	case ext.AnalyticsEvent:
		if v != "false" {
			setMetric(s, ext.EventSampleRate, 1)
		} else {
			setMetric(s, ext.EventSampleRate, 0)
		}
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

// TODO:
// test this
func getDatadogSpanName(s pdata.Span, datadogTags map[string]string) string {
	// largely a port of logic here
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L213
	// Get span name by using instrumentation and kind while backing off to span.kind
	if iln, ok := datadogTags[instrumentationLibraryName]; ok {
		return fmt.Sprintf("%s.%s", iln, s.Kind())
	}

	return fmt.Sprintf("%s.%s", "opentelemetry", s.Kind())
}

func getDatadogResourceName(s pdata.Span, datadogTags map[string]string) string {
	// largely a port of logic here
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L229
	// Get span resource name by checking for existence http.method + http.route 'GET /api'
	// backing off to just http.method, and then span.name if unrelated to http
	if method, methodOk := datadogTags[httpMethod]; methodOk {
		if route, routeOk := datadogTags[httpRoute]; routeOk {
			return fmt.Sprintf("%s %s", method, route)
		}

		return method
	}

	return s.Name()
}

func aggregateTracePayloadsByEnv(tracePayloads []*pb.TracePayload) []*pb.TracePayload {
	lookup := make(map[string]*pb.TracePayload)
	for _, tracePayload := range tracePayloads {
		key := fmt.Sprintf("%s|%s", tracePayload.HostName, tracePayload.Env)
		var existingPayload *pb.TracePayload
		if val, ok := lookup[key]; ok {
			existingPayload = val
		} else {
			existingPayload = &pb.TracePayload{
				HostName: tracePayload.HostName,
				Env:      tracePayload.Env,
				Traces:   make([]*pb.APITrace, 0),
			}
			lookup[key] = existingPayload
		}
		existingPayload.Traces = append(existingPayload.Traces, tracePayload.Traces...)
	}

	newPayloads := make([]*pb.TracePayload, 0)

	for _, tracePayload := range lookup {
		newPayloads = append(newPayloads, tracePayload)
	}
	return newPayloads
}
