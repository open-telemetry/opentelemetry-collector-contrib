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
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/traceutil"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/attributes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/utils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/tracetranslator"
)

const (
	keySamplingPriority string = "_sampling_priority_v1"
	keySamplingRate     string = "_sample_rate"
	versionTag          string = "version"
	oldILNameTag        string = "otel.instrumentation_library.name"
	currentILNameTag    string = "otel.library.name"
	errorCode           int32  = 1
	okCode              int32  = 0
	kindDb              string = "db"
	kindHTTP            string = "http"
	kindWeb             string = "web"
	kindCustom          string = "custom"
	kindCache           string = "cache"
	kindMemcached       string = "memcached"
	kindRedis           string = "redis"
	grpcPath            string = "grpc.path"
	eventsTag           string = "events"
	eventNameTag        string = "name"
	eventAttrTag        string = "attributes"
	eventTimeTag        string = "time"
	// maxMetaValLen value from
	// https://github.com/DataDog/datadog-agent/blob/140a4ee164261ef2245340c50371ba989fbeb038/pkg/trace/traceutil/truncate.go#L23.
	maxMetaValLen int = 5000
	// tagContainersTags specifies the name of the tag which holds key/value
	// pairs representing information about the container (Docker, EC2, etc).
	tagContainersTags = "_dd.tags.container"
	tagDatadogEnv     = "env"
)

// AttributeExceptionEventName the name of the exception event.
// TODO: Remove this when collector defines this semantic convention.
const AttributeExceptionEventName = "exception"

// converts Traces into an array of datadog trace payloads grouped by env
func convertToDatadogTd(td pdata.Traces, fallbackHost string, cfg *config.Config, blk *denylister, buildInfo component.BuildInfo) ([]*pb.TracePayload, []datadog.Metric) {
	// TODO:
	// do we apply other global tags, like version+service, to every span or only root spans of a service
	// should globalTags['service'] take precedence over a trace's resource.service.name? I don't believe so, need to confirm

	resourceSpans := td.ResourceSpans()

	var traces []*pb.TracePayload

	seenHosts := make(map[string]struct{})
	var series []datadog.Metric
	pushTime := pdata.NewTimestampFromTime(time.Now())

	spanNameMap := cfg.Traces.SpanNameRemappings

	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		host, ok := attributes.HostnameFromAttributes(rs.Resource().Attributes())
		if !ok {
			host = fallbackHost
		}

		seenHosts[host] = struct{}{}
		payload := resourceSpansToDatadogSpans(rs, host, cfg, blk, spanNameMap)

		traces = append(traces, &payload)
	}

	for host := range seenHosts {
		// Report the host as running
		runningMetric := metrics.DefaultMetrics("traces", host, uint64(pushTime), buildInfo)
		series = append(series, runningMetric...)
	}

	return traces, series
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
func resourceSpansToDatadogSpans(rs pdata.ResourceSpans, hostname string, cfg *config.Config, blk *denylister, spanNameMap map[string]string) pb.TracePayload {
	// get env tag
	env := utils.NormalizeTag(cfg.Env)

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
		payload.Env = utils.NormalizeTag(resourceEnv)
	}

	apiTraces := map[uint64]*pb.APITrace{}

	for i := 0; i < ilss.Len(); i++ {
		ils := ilss.At(i)
		extractInstrumentationLibraryTags(ils.InstrumentationLibrary(), datadogTags)
		spans := ils.Spans()
		for j := 0; j < spans.Len(); j++ {
			span := spanToDatadogSpan(spans.At(j), resourceServiceName, datadogTags, cfg, spanNameMap)
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
		// first drop trace if root span exists in trace chunk that is on denylist (drop trace no stats).
		// In the dd-agent, the denylist/blacklist behavior can be performed before most of the expensive
		// operations on the span.
		// See: https://github.com/DataDog/datadog-agent/blob/a6872e436681ea2136cf8a67465e99fdb4450519/pkg/trace/agent/agent.go#L200
		// However, in our case, the span must be converted from otlp span into a dd span first. This is for 2 reasons.
		// First, DD trace chunks rec'd by datadog-agent v0.4+ endpoint are expected as arrays of spans grouped by trace id.
		// But, since OTLP groups by arrays of spans from the same instrumentation library, not trace-id,
		// (contrib-instrumention-redis, contrib-instrumention-rails, etc), we have to iterate
		// over batch and group all spans by trace id.
		// Second, otlp->dd conversion is what creates the resource name that we are checking in the denylist.
		// Note: OTLP also groups by ResourceSpans but practically speaking a payload will usually only
		// contain 1 ResourceSpan array.

		// Root span is used to carry some trace-level metadata, such as sampling rate and priority.
		rootSpan := utils.GetRoot(apiTrace)

		if !blk.allows(rootSpan) {
			// drop trace by not adding to payload if it's root span matches denylist
			continue
		}

		// calculates analyzed spans for use in trace search and app analytics
		// appends a specific piece of metadata to these spans marking them as analyzed
		// TODO: allow users to configure specific spans to be marked as an analyzed spans for app analytics
		top := getAnalyzedSpans(apiTrace.Spans)

		// https://github.com/DataDog/datadog-agent/blob/443930a00bced1ea87d0067fc2f1fc3f98a48ba1/pkg/trace/agent/agent.go#L241-L243
		traceutil.ComputeTopLevel(apiTrace.Spans)

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
	spanNameMap map[string]string,
) *pb.Span {

	tags := aggregateSpanTags(s, datadogTags)

	// otel specification resource service.name takes precedence
	// and configuration DD_SERVICE as fallback if it exists
	if cfg.Service != "" {
		// prefer the collector level service name over an empty string or otel default
		if serviceName == "" || serviceName == tracetranslator.ResourceNoServiceName {
			serviceName = cfg.Service
		}
	}

	// peer.service should always be prioritized for service names when set because it is what the user decided.
	if peerService, ok := tags[conventions.AttributePeerService]; ok {
		serviceName = peerService
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
	startTime := s.StartTimestamp()
	endTime := s.EndTimestamp()
	duration := int64(endTime) - int64(startTime)

	// it's possible end time is unset, so default to 0 rather than using a negative number
	if s.EndTimestamp() == 0 {
		duration = 0
	}

	// by checking for error and setting error tags before creating datadog span
	// we can then set Error field when creating and predefine a max meta capacity
	isSpanError := getSpanErrorAndSetTags(s, tags)

	resourceName := getDatadogResourceName(s, tags)

	span := &pb.Span{
		TraceID:  decodeAPMTraceID(s.TraceID().Bytes()),
		SpanID:   decodeAPMSpanID(s.SpanID().Bytes()),
		Name:     remapDatadogSpanName(getDatadogSpanName(s, tags), spanNameMap),
		Resource: resourceName,
		Service:  normalizedServiceName,
		Start:    int64(startTime),
		Duration: duration,
		Metrics:  map[string]float64{},
		Meta:     make(map[string]string, len(tags)),
		Type:     inferDatadogType(s.Kind(), tags),
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
	// predefine capacity where possible with extra for _dd.tags.container payload and duplicate env tag
	datadogTags = make(map[string]string, attrs.Len()+2)

	if attrs.Len() == 0 {
		return tracetranslator.ResourceNoServiceName, datadogTags
	}

	attrs.Range(func(k string, v pdata.AttributeValue) bool {
		datadogTags[k] = v.AsString()
		return true
	})

	// specification states that the resource level deployment.environment should be used for passing env,
	// and also a number of Datadog UI components are hardcoded to point to /  look for / search with `env`.
	// So we ensure that anytime deployment.environment is set, `env` is the same value.
	// In the future, we can probably just set `env` using the deployment.environment value, but drop the duplicate
	// deployment.environment tag afterward. Without knowing whether this would break existing user setups, its better
	// to simply set both tags.
	// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.4.0/specification/resource/semantic_conventions/deployment_environment.md#deployment
	if resourceEnv, ok := datadogTags[conventions.AttributeDeploymentEnvironment]; ok {
		datadogTags[tagDatadogEnv] = utils.NormalizeTag(resourceEnv)
	}

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
		datadogTags[conventions.InstrumentationLibraryName] = ilName
	}
	if ilVer := il.Version(); ilVer != "" {
		datadogTags[conventions.InstrumentationLibraryVersion] = ilVer
	}
}

func aggregateSpanTags(span pdata.Span, datadogTags map[string]string) map[string]string {
	// predefine capacity as at most the size attributes and global tags
	// there may be overlap between the two.
	spanTags := make(map[string]string, span.Attributes().Len()+len(datadogTags))

	for key, val := range datadogTags {
		spanTags[utils.NormalizeTag(key)] = val
	}

	span.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch k {
		case keySamplingPriority:
			spanTags[k] = v.AsString()
		case keySamplingRate:
			spanTags[k] = v.AsString()
		default:
			spanTags[utils.NormalizeTag(k)] = v.AsString()
		}

		return true
	})

	// we don't want to normalize these tags since `_dd` is a special case
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
	if val, ok := spanTags[conventions.AttributeK8SPodName]; ok {
		b.WriteString(fmt.Sprintf("%s:%s,", "pod_name", val))
	}

	if val, ok := spanTags[conventions.AttributeAWSECSTaskARN]; ok {
		b.WriteString(fmt.Sprintf("%s:%s,", "task_arn", val))
	}

	return strings.TrimSuffix(b.String(), ",")
}

// inferDatadogTypes returns a string for the datadog type based on metadata
// in the otel span. DB semantic conventions state that what datadog
// would mark as a db or cache span type, otel marks as a CLIENT span kind, but
// has a required attribute 'db.system'. This is the only required attribute for
// db or cache spans, so we can reliably use it as a heuristic for infering the
// difference between client, db, and cache span types. The only "cache" spans
// in datadog currently are redis and memcached, so those are the only two db.system
// attribute values we have to check to determine whether it's a db or cache span
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/database.md#semantic-conventions-for-database-client-calls
func inferDatadogType(kind pdata.SpanKind, datadogTags map[string]string) string {
	switch kind {
	case pdata.SpanKindClient:
		if dbSysOtlp, ok := datadogTags[conventions.AttributeDBSystem]; ok {
			if dbSysOtlp == kindRedis || dbSysOtlp == kindMemcached {
				return kindCache
			}

			return kindDb
		}

		return kindHTTP
	case pdata.SpanKindServer:
		return kindWeb
	default:
		return kindCustom
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
	if len(v) > maxMetaValLen {
		v = utils.TruncateUTF8(v, maxMetaValLen)
	}

	switch key {
	// if a span has `service.name` set as the tag
	case ext.ServiceName:
		s.Service = utils.NormalizeTag(v)
	case ext.SpanType:
		s.Type = utils.NormalizeTag(v)
	case ext.AnalyticsEvent:
		// we dont want to normalize ints
		if v != "false" {
			setMetric(s, ext.EventSampleRate, 1)
		} else {
			setMetric(s, ext.EventSampleRate, 0)
		}
	case keySamplingRate:
		if sampleRateFlt, err := strconv.ParseFloat(v, 64); err == nil {
			setMetric(s, keySamplingRate, sampleRateFlt)
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
	if ilnOtlp, okOtlp := datadogTags[conventions.InstrumentationLibraryName]; okOtlp {
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

		// if the error message doesnt exist, try to infer useful error information from
		// the status message, and http status code / http status text.
		// if we find useful error.message info, also set a fallback error.type
		// otherwise leave these tags unset and allow the UI to handle defaults
		if _, ok := tags[ext.ErrorMsg]; !ok {
			if status.Message() != "" {
				tags[ext.ErrorMsg] = status.Message()
				// look for useful http metadata if it exists and add that as a fallback for the error message
			} else if statusCode, ok := tags[conventions.AttributeHTTPStatusCode]; ok {
				if statusText, ok := tags["http.status_text"]; ok {
					tags[ext.ErrorMsg] = fmt.Sprintf("%s %s", statusCode, statusText)
				} else {
					tags[ext.ErrorMsg] = statusCode
				}
			}

			// If we weren't able to pull an error type, but we do have an error message
			// set to a reasonable default
			if (tags[ext.ErrorType] == "") && (tags[ext.ErrorMsg] != "") {
				tags[ext.ErrorType] = "error"
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
			} else if s.Kind() == pdata.SpanKindClient && httpStatusCode >= 400 {
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
//  https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/exceptions.md#attributes
func extractErrorTagsFromEvents(s pdata.Span, tags map[string]string) {
	evts := s.Events()
	for i := evts.Len() - 1; i >= 0; i-- {
		evt := evts.At(i)
		if evt.Name() == AttributeExceptionEventName {
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
		event[eventAttrTag] = spanEvent.Attributes().AsRaw()
		eventArray = append(eventArray, event)
	}
	eventArrayBytes, _ := json.Marshal(&eventArray)
	return string(eventArrayBytes)
}

// remapDatadogSpanName allows users to map their datadog span operation names to
// another string as they see fit.
func remapDatadogSpanName(name string, spanNameMap map[string]string) string {
	if updatedSpanName := spanNameMap[name]; updatedSpanName != "" {
		return updatedSpanName
	}

	return name
}
