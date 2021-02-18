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
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	tracetranslator "go.opentelemetry.io/collector/translator/trace"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils"
)

const (
	keySamplingPriority string = "_sampling_priority_v1"
	versionTag          string = "version"
	oldILNameTag        string = "otel.instrumentation_library.name"
	currentILNameTag    string = "otel.library.name"
	errorCode           int32  = 1
	okCode              int32  = 0
	httpKind            string = "http"
	webKind             string = "web"
	customKind          string = "custom"
	grpcPath            string = "grpc.path"
	eventsTag           string = "events"
	eventNameTag        string = "name"
	eventAttrTag        string = "attributes"
	eventTimeTag        string = "time"
	// tagContainersTags specifies the name of the tag which holds key/value
	// pairs representing information about the container (Docker, EC2, etc).
	tagContainersTags = "_dd.tags.container"
)

// converts Traces into an array of datadog trace payloads grouped by env
func convertToDatadogTd(td pdata.Traces, calculator *sublayerCalculator, cfg *config.Config) ([]*pb.TracePayload, []datadog.Metric) {
	// TODO:
	// do we apply other global tags, like version+service, to every span or only root spans of a service
	// should globalTags['service'] take precedence over a trace's resource.service.name? I don't believe so, need to confirm

	resourceSpans := td.ResourceSpans()

	var traces []*pb.TracePayload

	var runningMetrics []datadog.Metric
	pushTime := time.Now().UTC().UnixNano()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		// TODO pass logger here once traces code stabilizes
		hostname := *metadata.GetHost(zap.NewNop(), cfg)
		resHostname, ok := metadata.HostnameFromAttributes(rs.Resource().Attributes())
		if ok {
			hostname = resHostname
		}

		payload := resourceSpansToDatadogSpans(rs, calculator, hostname, cfg)
		traces = append(traces, &payload)

		ms := metrics.DefaultMetrics("traces", uint64(pushTime))
		ms[0].Host = &hostname
		runningMetrics = append(runningMetrics, ms...)
	}

	return traces, runningMetrics
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
				Traces:   make([]*pb.APITrace, 0, len(tracePayload.Traces)),
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

// converts a Trace's resource spans into a trace payload
func resourceSpansToDatadogSpans(rs pdata.ResourceSpans, calculator *sublayerCalculator, hostname string, cfg *config.Config) pb.TracePayload {
	// get env tag
	env := cfg.Env

	resource := rs.Resource()
	ilss := rs.InstrumentationLibrarySpans()

	payload := pb.TracePayload{
		HostName:     hostname,
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	if resource.Attributes().Len() == 0 && ilss.Len() == 0 {
		return payload
	}

	resourceServiceName, datadogTags := resourceToDatadogServiceNameAndAttributeMap(resource)

	// specification states that the resource level deployment.environment should be used for passing env, so defer to that
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/resource/semantic_conventions/deployment_environment.md#deployment
	if resourceEnv, ok := datadogTags[conventions.AttributeDeploymentEnvironment]; ok {
		payload.Env = resourceEnv
	}

	apiTraces := map[uint64]*pb.APITrace{}

	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		extractInstrumentationLibraryTags(ils.InstrumentationLibrary(), datadogTags)
		spans := ils.Spans()
		for j := 0; j < spans.Len(); j++ {
			span := spanToDatadogSpan(spans.At(j), resourceServiceName, datadogTags, cfg)
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
		// calculates analyzed spans for use in trace search and app analytics
		// appends a specific piece of metadata to these spans marking them as analyzed
		// TODO: allow users to configure specific spans to be marked as an analyzed spans for app analytics
		top := getAnalyzedSpans(apiTrace.Spans)

		// calculates span metrics for representing direction and timing among it's different services for display in
		// service overview graphs
		// see: https://github.com/DataDog/datadog-agent/blob/f69a7d35330c563e9cad4c5b8865a357a87cd0dc/pkg/trace/stats/sublayers.go#L204
		computeSublayerMetrics(calculator, apiTrace.Spans)
		payload.Transactions = append(payload.Transactions, top...)
		payload.Traces = append(payload.Traces, apiTrace)
	}

	return payload
}

// convertSpan takes an internal span representation and returns a Datadog span.
func spanToDatadogSpan(s pdata.Span,
	serviceName string,
	datadogTags map[string]string,
	cfg *config.Config,
) *pb.Span {

	tags := aggregateSpanTags(s, datadogTags)

	// otel specification resource service.name takes precedence
	// and configuration DD_ENV as fallback if it exists
	if cfg.Service != "" {
		// prefer the collector level service name over an empty string or otel default
		if serviceName == "" || serviceName == tracetranslator.ResourceNoServiceName {
			serviceName = cfg.Service
		}
	}

	normalizedServiceName := utils.NormalizeServiceName(serviceName)

	//  canonical resource attribute version should override others if it exists
	if rsTagVersion := tags[conventions.AttributeServiceVersion]; rsTagVersion != "" {
		tags[versionTag] = rsTagVersion
	} else {
		// if no version tag exists, set it if provided via config
		if cfg.Version != "" {
			if tagVersion := tags[versionTag]; tagVersion == "" {
				tags[versionTag] = cfg.Version
			}
		}
	}

	// get tracestate as just a general tag
	if len(s.TraceState()) > 0 {
		tags[tracetranslator.TagW3CTraceState] = string(s.TraceState())
	}

	// get events as just a general tag
	if s.Events().Len() > 0 {
		tags[eventsTag] = eventsToString(s.Events())
	}

	// get start/end time to calc duration
	startTime := s.StartTime()
	endTime := s.EndTime()
	duration := int64(endTime) - int64(startTime)

	// it's possible end time is unset, so default to 0 rather than using a negative number
	if s.EndTime() == 0 {
		duration = 0
	}

	// by checking for error and setting error tags before creating datadog span
	// we can then set Error field when creating and predefine a max meta capacity
	isSpanError := getSpanErrorAndSetTags(s, tags)

	span := &pb.Span{
		TraceID:  decodeAPMTraceID(s.TraceID().Bytes()),
		SpanID:   decodeAPMSpanID(s.SpanID().Bytes()),
		Name:     getDatadogSpanName(s, tags),
		Resource: getDatadogResourceName(s, tags),
		Service:  normalizedServiceName,
		Start:    int64(startTime),
		Duration: duration,
		Metrics:  map[string]float64{},
		Meta:     make(map[string]string, len(tags)),
		Type:     spanKindToDatadogType(s.Kind()),
		Error:    isSpanError,
	}

	if !s.ParentSpanID().IsEmpty() {
		span.ParentID = decodeAPMSpanID(s.ParentSpanID().Bytes())
	}

	// Set Attributes as Tags
	for key, val := range tags {
		setStringTag(span, key, val)
	}

	return span
}

func resourceToDatadogServiceNameAndAttributeMap(
	resource pdata.Resource,
) (serviceName string, datadogTags map[string]string) {
	attrs := resource.Attributes()
	// predefine capacity where possible with extra for _dd.tags.container payload
	datadogTags = make(map[string]string, attrs.Len()+1)

	if attrs.Len() == 0 {
		return tracetranslator.ResourceNoServiceName, datadogTags
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
	if ilName := il.Name(); ilName != "" {
		datadogTags[tracetranslator.TagInstrumentationName] = ilName
	}
	if ilVer := il.Version(); ilVer != "" {
		datadogTags[tracetranslator.TagInstrumentationVersion] = ilVer
	}
}

func aggregateSpanTags(span pdata.Span, datadogTags map[string]string) map[string]string {
	// predefine capacity as at most the size attributes and global tags
	// there may be overlap between the two.
	spanTags := make(map[string]string, span.Attributes().Len()+len(datadogTags))

	for key, val := range datadogTags {
		spanTags[key] = val
	}

	span.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		spanTags[k] = tracetranslator.AttributeValueToString(v, false)

	})

	spanTags[tagContainersTags] = buildDatadogContainerTags(spanTags)
	return spanTags
}

// buildDatadogContainerTags returns container and orchestrator tags belonging to containerID
// as a comma delimeted list for datadog's special container tag key
func buildDatadogContainerTags(spanTags map[string]string) string {
	var b strings.Builder

	if val, ok := spanTags[conventions.AttributeContainerID]; ok {
		b.WriteString(fmt.Sprintf("%s:%s,", "container_id", val))
	}
	if val, ok := spanTags[conventions.AttributeK8sPod]; ok {
		b.WriteString(fmt.Sprintf("%s:%s,", "pod_name", val))
	}

	return strings.TrimSuffix(b.String(), ",")
}

// TODO: some clients send SPAN_KIND_UNSPECIFIED for valid kinds
// we also need a more formal mapping for cache and db types
func spanKindToDatadogType(kind pdata.SpanKind) string {
	switch kind {
	case pdata.SpanKindCLIENT:
		return httpKind
	case pdata.SpanKindSERVER:
		return webKind
	default:
		return customKind
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

func decodeAPMSpanID(rawID [8]byte) uint64 {
	return decodeAPMId(hex.EncodeToString(rawID[:]))
}

func decodeAPMTraceID(rawID [16]byte) uint64 {
	return decodeAPMId(hex.EncodeToString(rawID[:]))
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

func getDatadogSpanName(s pdata.Span, datadogTags map[string]string) string {
	// largely a port of logic here
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L213
	// Get span name by using instrumentation library name and span kind while backing off to span.kind

	// The spec has changed over time and, depending on the original exporter, IL Name could represented a few different ways
	// so we try to account for all permutations
	if ilnOtlp, okOtlp := datadogTags[tracetranslator.TagInstrumentationName]; okOtlp {
		return utils.NormalizeSpanName(fmt.Sprintf("%s.%s", ilnOtlp, utils.NormalizeSpanKind(s.Kind())), false)
	}

	if ilnOtelCur, okOtelCur := datadogTags[currentILNameTag]; okOtelCur {
		return utils.NormalizeSpanName(fmt.Sprintf("%s.%s", ilnOtelCur, utils.NormalizeSpanKind(s.Kind())), false)
	}

	if ilnOtelOld, okOtelOld := datadogTags[oldILNameTag]; okOtelOld {
		return utils.NormalizeSpanName(fmt.Sprintf("%s.%s", ilnOtelOld, utils.NormalizeSpanKind(s.Kind())), false)
	}

	return utils.NormalizeSpanName(fmt.Sprintf("%s.%s", "opentelemetry", utils.NormalizeSpanKind(s.Kind())), false)
}

func getDatadogResourceName(s pdata.Span, datadogTags map[string]string) string {
	// largely a port of logic here
	// https://github.com/open-telemetry/opentelemetry-python/blob/b2559409b2bf82e693f3e68ed890dd7fd1fa8eae/exporter/opentelemetry-exporter-datadog/src/opentelemetry/exporter/datadog/exporter.py#L229
	// Get span resource name by checking for existence http.method + http.route 'GET /api'
	// Also check grpc path as fallback for http requests
	// backing off to just http.method, and then span.name if unrelated to http
	if method, methodOk := datadogTags[conventions.AttributeHTTPMethod]; methodOk {
		if route, routeOk := datadogTags[conventions.AttributeHTTPRoute]; routeOk {
			return fmt.Sprintf("%s %s", method, route)
		}

		if grpcRoute, grpcRouteOk := datadogTags[grpcPath]; grpcRouteOk {
			return fmt.Sprintf("%s %s", method, grpcRoute)
		}

		return method
	}

	//add resource conventions for messaging queues, operaton + destination
	if msgOperation, msgOperationOk := datadogTags[conventions.AttributeMessagingOperation]; msgOperationOk {
		if destination, destinationOk := datadogTags[conventions.AttributeMessagingDestination]; destinationOk {
			return fmt.Sprintf("%s %s", msgOperation, destination)
		}

		return msgOperation
	}

	// add resource convention for rpc services , method+service, fallback to just method if no service attribute
	if rpcMethod, rpcMethodOk := datadogTags[conventions.AttributeRPCMethod]; rpcMethodOk {
		if rpcService, rpcServiceOk := datadogTags[conventions.AttributeRPCService]; rpcServiceOk {
			return fmt.Sprintf("%s %s", rpcMethod, rpcService)
		}

		return rpcMethod
	}

	return s.Name()
}

func getSpanErrorAndSetTags(s pdata.Span, tags map[string]string) int32 {
	var isError int32
	// Set Span Status and any response or error details
	status := s.Status()
	switch status.Code() {
	case pdata.StatusCodeOk:
		isError = okCode
	case pdata.StatusCodeError:
		isError = errorCode
	default:
		isError = okCode
	}

	if isError == errorCode {
		extractErrorTagsFromEvents(s, tags)
		// If we weren't able to pull an error type or message, go ahead and set
		// these to the old defaults
		if _, ok := tags[ext.ErrorType]; !ok {
			tags[ext.ErrorType] = "ERR_CODE_" + strconv.FormatInt(int64(status.Code()), 10)
		}

		if _, ok := tags[ext.ErrorMsg]; !ok {
			if status.Message() != "" {
				tags[ext.ErrorMsg] = status.Message()
			} else {
				tags[ext.ErrorMsg] = "ERR_CODE_" + strconv.FormatInt(int64(status.Code()), 10)
			}
		}
	}

	// if status code exists check if error depending on type
	if tags[conventions.AttributeHTTPStatusCode] != "" {
		httpStatusCode, err := strconv.ParseInt(tags[conventions.AttributeHTTPStatusCode], 10, 64)
		if err == nil {
			// for 500 type, always mark as error
			if httpStatusCode >= 500 {
				isError = errorCode
				// for 400 type, mark as error if it is an http client
			} else if s.Kind() == pdata.SpanKindCLIENT && httpStatusCode >= 400 {
				isError = errorCode
			}
		}
	}

	return isError
}

// Finds the last exception event in the span, and surfaces it to DataDog. DataDog spans only support a single
// exception per span, but otel supports many exceptions as "Events" on a given span. The last exception was
// chosen for now as most otel-instrumented libraries (http, pg, etc.) only capture a single exception (if any)
// per span. If multiple exceptions are logged, it's my assumption that the last exception is most likely the
// exception that escaped the scope of the span.
//
// TODO:
//  Seems that the spec has an attribute that hasn't made it to the collector yet -- "exception.escaped".
//  This seems optional (SHOULD vs. MUST be set), but it's likely that we want to bubble up the exception
//  that escaped the scope of the span ("exception.escaped" == true) instead of the last exception event
//  in the case that these events differ.
//
//  https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/exceptions.md#attributes
func extractErrorTagsFromEvents(s pdata.Span, tags map[string]string) {
	evts := s.Events()
	for i := evts.Len() - 1; i >= 0; i-- {
		evt := evts.At(i)
		if evt.Name() == conventions.AttributeExceptionEventName {
			attribs := evt.Attributes()
			if errType, ok := attribs.Get(conventions.AttributeExceptionType); ok {
				tags[ext.ErrorType] = errType.StringVal()
			}
			if errMsg, ok := attribs.Get(conventions.AttributeExceptionMessage); ok {
				tags[ext.ErrorMsg] = errMsg.StringVal()
			}
			if errStack, ok := attribs.Get(conventions.AttributeExceptionStacktrace); ok {
				tags[ext.ErrorStack] = errStack.StringVal()
			}
			return
		}
	}
}

// Convert Span Events to a string so that they can be appended to the span as a tag.
// Span events are probably better served as Structured Logs sent to the logs API
// with the trace id and span id added for log/trace correlation. However this would
// mean a separate API intake endpoint and also Logs and Traces may not be enabled for
// a user, so for now just surfacing this information as a string is better than not
// including it at all. The tradeoff is that this increases the size of the span and the
// span may have a tag that exceeds max size allowed in backend/ui/etc.
//
// TODO: Expose configuration option for collecting Span Events as Logs within Datadog
// and add forwarding to Logs API intake.
func eventsToString(evts pdata.SpanEventSlice) string {
	eventArray := make([]map[string]interface{}, 0, evts.Len())
	for i := 0; i < evts.Len(); i++ {
		spanEvent := evts.At(i)
		event := map[string]interface{}{}
		event[eventNameTag] = spanEvent.Name()
		event[eventTimeTag] = spanEvent.Timestamp()
		event[eventAttrTag] = tracetranslator.AttributeMapToMap(spanEvent.Attributes())
		eventArray = append(eventArray, event)
	}
	eventArrayBytes, _ := json.Marshal(&eventArray)
	return string(eventArrayBytes)
}
