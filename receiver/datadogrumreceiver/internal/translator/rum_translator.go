package translator

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

// Attribute prefixes
const prefixDatadog = "datadog."
const prefixUser = "user."
const prefixGeo = "geo."
const prefixDevice = "device."
const prefixOs = "os."
const prefixSession = "session."
const prefixView = "view."
const prefixResource = "resource."
const prefixAction = "action."

// User attribute names
const attrID = "id"
const attrName = "name"
const attrEmail = "email"

// Application attribute names
const attrApplicationID = "id"
const attrApplicationName = "name"

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func GetBuffer() *bytes.Buffer {
	buffer := bufferPool.Get().(*bytes.Buffer)
	buffer.Reset()
	return buffer
}

func PutBuffer(buffer *bytes.Buffer) {
	bufferPool.Put(buffer)
}

type RUMPayload struct {
	Type string
}

func parseW3CTraceContext(traceparent string) (traceID pcommon.TraceID, spanID pcommon.SpanID, err error) {
	// W3C traceparent format: version-traceID-spanID-flags
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("invalid traceparent format: %s", traceparent)
	}

	// Parse trace ID (32 hex characters)
	traceIDBytes, err := hex.DecodeString(parts[1])
	if err != nil || len(traceIDBytes) != 16 {
		return pcommon.NewTraceIDEmpty(), pcommon.SpanID{}, fmt.Errorf("invalid trace ID: %s", parts[1])
	}
	copy(traceID[:], traceIDBytes)

	// Parse span ID
	spanIDBytes, err := hex.DecodeString(parts[2])
	if err != nil || len(spanIDBytes) != 8 {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("invalid parent ID: %s", parts[2])
	}
	copy(spanID[:], spanIDBytes)

	return traceID, spanID, nil
}

/*
func parseIDs(payload map[string]any, req *http.Request) (traceID uint64, spanID uint64, err error) {
	ddMetadata, ok := payload["_dd"].(map[string]any)
	if !ok {
		return 0, 0, fmt.Errorf("failed to find _dd metadata in payload")
	}

	traceIDString, ok := ddMetadata["trace_id"].(string)
	if !ok {
		return 0, 0, fmt.Errorf("failed to retrieve traceID from payload")
	}
	traceID, err = strconv.ParseUint(traceIDString, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse traceID: %w", err)
	}

	spanIDString, ok := ddMetadata["span_id"].(string)
	if !ok {
		return 0, 0, fmt.Errorf("failed to retrieve spanID from payload")
	}
	spanID, err = strconv.ParseUint(spanIDString, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse spanID: %w", err)
	}

	return traceID, spanID, nil
}
*/

func setNumberAttributes(span ptrace.Span, data map[string]any, prefix string, attributes []string) {
	for _, attribute := range attributes {
		if value, ok := data[attribute].(float64); ok {
			span.Attributes().PutInt(prefix+attribute, int64(value))
		}
	}
}

func setStringAttributes(span ptrace.Span, data map[string]any, prefix string, attributes []string) {
	for _, attribute := range attributes {
		if value, ok := data[attribute].(string); ok {
			span.Attributes().PutStr(prefix+attribute, value)
		}
	}
}

func setSession(newSpan ptrace.Span, session map[string]any) {
	if session == nil {
		return
	}

	if timeSpent, ok := session["time_spent"].(float64); ok {
		newSpan.Attributes().PutInt("datadog.session.time_spent", int64(timeSpent))
	}

	countCategories := []string{
		"error",
		"long_task",
		"resource",
		"action",
		"frustration",
	}
	for _, category := range countCategories {
		if countCategory, ok := session[category].(map[string]any); ok {
			setNumberAttributes(newSpan, countCategory, "datadog.session."+category+".", []string{"count"})
		}
	}

	if id, ok := session["id"].(string); ok {
		newSpan.Attributes().PutStr("session.id", id)
	}
	if ip, ok := session["ip"].(string); ok {
		newSpan.Attributes().PutStr("client.address", ip)
	}

	if isActive, ok := session["is_active"].(bool); ok {
		newSpan.Attributes().PutBool("datadog.session.is_active", isActive)
	}

	sessionAttributes := []string{
		"type",
		"referrer",
	}
	setStringAttributes(newSpan, session, "datadog.session.", sessionAttributes)

	if initialView, ok := session["initial_view"].(map[string]any); ok {
		initialViewAttributes := []string{
			"id",
			"url_host",
			"url_path",
			"url_path_group",
		}
		setStringAttributes(newSpan, initialView, "datadog.session.initial_view.", initialViewAttributes)

		if urlQuery, ok := initialView["url_query"].(map[string]any); ok {
			queryMap := newSpan.Attributes().PutEmptyMap("datadog.session.initial_view.url_query")
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := initialView["url_scheme"].(map[string]any); ok {
			schemeMap := newSpan.Attributes().PutEmptyMap("datadog.session.initial_view.url_scheme")
			for key, value := range urlScheme {
				if strValue, ok := value.(string); ok {
					schemeMap.PutStr(key, strValue)
				}
			}
		}
	}

	if lastView, ok := session["last_view"].(map[string]any); ok {
		lastViewAttributes := []string{
			"id",
			"url_host",
			"url_path",
			"url_path_group",
		}
		setStringAttributes(newSpan, lastView, "datadog.session.last_view.", lastViewAttributes)

		if urlQuery, ok := lastView["url_query"].(map[string]any); ok {
			queryMap := newSpan.Attributes().PutEmptyMap("datadog.session.last_view.url_query")
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := lastView["url_scheme"].(map[string]any); ok {
			schemeMap := newSpan.Attributes().PutEmptyMap("datadog.session.last_view.url_scheme")
			for key, value := range urlScheme {
				if strValue, ok := value.(string); ok {
					schemeMap.PutStr(key, strValue)
				}
			}
		}
	}
}

func setView(newSpan ptrace.Span, view map[string]any) {
	if view == nil {
		return
	}

	viewMetrics := []string{
		"time_spent",
		"first_byte",
		"largest_contentful_paint",
		"first_input_delay",
		"interaction_to_next_paint",
		"cumulative_layout_shift",
		"loading_time",
		"first_contentful_paint",
		"dom_interactive",
		"dom_content_loaded",
		"dom_complete",
		"load_event",
	}
	setNumberAttributes(newSpan, view, "datadog.view.", viewMetrics)

	countCategories := []string{
		"error",
		"long_task",
		"resource",
		"action",
		"frustration",
	}
	for _, category := range countCategories {
		if countCategory, ok := view[category].(map[string]any); ok {
			setNumberAttributes(newSpan, countCategory, "datadog.view."+category+".", []string{"count"})
		}
	}
	//TO-DO: create mappings doc on confluence
	viewAttributes := []string{
		"id",
		"loading_type",
		"referrer",
		"url",
		"url_hash",
		"url_host",
		"url_path",
		"url_path_group",
		"largest_contentful_paint_target_selector",
		"first_input_delay_target_selector",
		"interaction_to_next_paint_target_selector",
		"cumulative_layout_shift_target_selector",
	}
	setStringAttributes(newSpan, view, "datadog.view.", viewAttributes)

	if urlQuery, ok := view["url_query"].(map[string]any); ok {
		urlQueryAttributes := []string{
			"utm_source",
			"utm_medium",
			"utm_campaign",
			"utm_content",
			"utm_term",
		}
		setStringAttributes(newSpan, urlQuery, "datadog.view.url_query.", urlQueryAttributes)
	}

	if urlScheme, ok := view["url_scheme"].(map[string]any); ok {
		schemeMap := newSpan.Attributes().PutEmptyMap("datadog.view.url_scheme")
		for key, value := range urlScheme {
			if strValue, ok := value.(string); ok {
				schemeMap.PutStr(key, strValue)
			}
		}
	}
}

// TO-DO: make constants for attribute names
func setResource(newSpan ptrace.Span, resource map[string]any) {
	if resource == nil {
		return
	}

	resourceMetrics := []string{
		"duration",
		"size",
		"status_code",
	}
	setNumberAttributes(newSpan, resource, "datadog.resource.", resourceMetrics)

	durationCategories := []string{
		"ssl",
		"dns",
		"redirect",
		"first_byte",
		"download",
	}
	for _, category := range durationCategories {
		if durationCategory, ok := resource[category].(map[string]any); ok {
			setNumberAttributes(newSpan, durationCategory, "datadog.resource."+category+".", []string{"duration"})
		}
	}

	resourceAttributes := []string{
		"type",
		"method",
		"url",
		"url_host",
		"url_path",
		"url_scheme",
	}
	setStringAttributes(newSpan, resource, "ur", resourceAttributes)

	if urlQuery, ok := resource["url_query"].(map[string]any); ok {
		queryMap := newSpan.Attributes().PutEmptyMap("datadog.resource.url_query")
		for key, value := range urlQuery {
			if strValue, ok := value.(string); ok {
				queryMap.PutStr(key, strValue)
			}
		}
	}

	if provider, ok := resource["provider"].(map[string]any); ok {
		providerAttributes := []string{
			"name",
			"domain",
			"type",
		}
		setStringAttributes(newSpan, provider, "datadog.resource.provider.", providerAttributes)
	}
}

func setError(newSpan ptrace.Span, error map[string]any) {
	if error == nil {
		return
	}

	ddErrorAttributes := []string{
		"source",
		"stack",
	}
	setStringAttributes(newSpan, error, "datadog.error.", ddErrorAttributes)

	errorAttributes := []string{
		"type",
		"message",
	}
	setStringAttributes(newSpan, error, "error.", errorAttributes)
}

func setAction(newSpan ptrace.Span, action map[string]any) {
	if action == nil {
		return
	}

	if loadingTime, ok := action["loading_time"].(float64); ok {
		newSpan.Attributes().PutInt("datadog.action.loading_time", int64(loadingTime))
	}

	countCategories := []string{
		"error",
		"long_task",
		"resource",
	}
	for _, category := range countCategories {
		if countCategory, ok := action[category].(map[string]any); ok {
			setNumberAttributes(newSpan, countCategory, "datadog.action."+category+".", []string{"count"})
		}
	}

	actionAttributes := []string{
		"id",
		"type",
		"name",
	}
	setStringAttributes(newSpan, action, "datadog.action.", actionAttributes)

	if target, ok := action["target"].(map[string]any); ok {
		setStringAttributes(newSpan, target, "datadog.action.target.", []string{"name"})
	}

	if frustration, ok := action["frustration"].(map[string]any); ok {
		frustrationAttributes := []string{
			"type:dead_click",
			"type:rage_click",
			"type:error_click",
		}
		setStringAttributes(newSpan, frustration, "datadog.action.frustration.", frustrationAttributes)
	}
}

func setGeo(newSpan ptrace.Span, geo map[string]any) {
	if geo == nil {
		return
	}

	if countryIsoCode, ok := geo["country_iso_code"].(string); ok {
		newSpan.Attributes().PutStr("geo.country.iso_code", countryIsoCode)
	}

	if city, ok := geo["city"].(string); ok {
		newSpan.Attributes().PutStr("geo.locality.name", city)
	}

	if continentCode, ok := geo["continent_code"].(string); ok {
		newSpan.Attributes().PutStr("geo.continent_code", continentCode)
	}

	geoAttributes := []string{
		"country",
		"country_subdivision",
		"continent",
	}
	setStringAttributes(newSpan, geo, "datadog.geo.", geoAttributes)
}

func ToLogs(payload map[string]any, req *http.Request, reqBytes []byte) plog.Logs {
	results := plog.NewLogs()
	rl := results.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl(semconv.SchemaURL)
	ParseRUMRequestIntoResource(rl.Resource(), payload, req, reqBytes)

	in := rl.ScopeLogs().AppendEmpty()
	in.Scope().SetName("Datadog")

	newLogRecord := in.LogRecords().AppendEmpty()
	newLogRecord.Attributes().PutBool("datadog.is_rum", true)

	fmt.Println("%%%%% SUCCESSFUL LOG PARSE")
	return results
}

func ToTraces(payload map[string]any, req *http.Request, reqBytes []byte) ptrace.Traces {
	results := ptrace.NewTraces()
	rs := results.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl(semconv.SchemaURL)
	ParseRUMRequestIntoResource(rs.Resource(), payload, req, reqBytes)

	in := rs.ScopeSpans().AppendEmpty()
	in.Scope().SetName("Datadog")

	traceparent := req.Header.Get("traceparent")
	if traceparent == "" {
		return results
	}
	traceID, spanID, err := parseW3CTraceContext(traceparent)
	if err != nil {
		fmt.Println(err)
		return results
	}

	newSpan := in.Spans().AppendEmpty()
	newSpan.SetName("RUMResource")
	newSpan.SetTraceID(traceID)
	newSpan.SetSpanID(spanID)
	newSpan.Attributes().PutBool("datadog.is_rum", true)

	if date, ok := payload["date"].(float64); ok {
		newSpan.Attributes().PutInt("datadog.date", int64(date))
	}

	if device, ok := payload["device"].(string); ok {
		newSpan.Attributes().PutStr("datadog.device", device)
	}

	if service, ok := payload["service"].(string); ok {
		newSpan.Attributes().PutStr("datadog.service", service)
	}

	if user, ok := payload["usr"].(map[string]any); ok {
		userAttributes := []string{
			"id",
			"name",
			"email",
		}
		setStringAttributes(newSpan, user, "user.", userAttributes)
	}

	if eventType, ok := payload["type"].(string); ok {
		newSpan.Attributes().PutStr("datadog.type", eventType)
	}

	if application, ok := payload["application"].(map[string]any); ok {
		applicationAttributes := []string{
			"id",
			"name",
		}
		setStringAttributes(newSpan, application, "datadog.application.", applicationAttributes)
	}

	//TO-DO: device.type and device.name
	if device, ok := payload["device"].(map[string]any); ok {
		if deviceType, ok := device["type"].(string); ok {
			newSpan.Attributes().PutStr("datadog.device.type", deviceType)
		}
		if brand, ok := device["brand"].(string); ok {
			newSpan.Attributes().PutStr("device.manufacturer", brand)
		}
		if model, ok := device["model"].(string); ok {
			newSpan.Attributes().PutStr("device.model.identifier", model)
		}
		if name, ok := device["name"].(string); ok {
			newSpan.Attributes().PutStr("device.model.name", name)
		}
	}

	if os, ok := payload["os"].(map[string]any); ok {
		osAttributes := []string{
			"name",
			"version",
		}
		setStringAttributes(newSpan, os, "os.", osAttributes)

		setStringAttributes(newSpan, os, "datadog.os.", []string{"version_major"})
	}

	if geo, ok := payload["geo"].(map[string]any); ok {
		setGeo(newSpan, geo)
	}

	if session, ok := payload["session"].(map[string]any); ok {
		setSession(newSpan, session)
	}

	if view, ok := payload["view"].(map[string]any); ok {
		setView(newSpan, view)
	}

	if resource, ok := payload["resource"].(map[string]any); ok {
		setResource(newSpan, resource)
	}

	if longTask, ok := payload["long_task"].(map[string]any); ok {
		if duration, ok := longTask["duration"].(float64); ok {
			newSpan.Attributes().PutInt("datadog.long_task.duration", int64(duration))
		}
	}

	if error, ok := payload["error"].(map[string]any); ok {
		setError(newSpan, error)
	}

	if action, ok := payload["action"].(map[string]any); ok {
		setAction(newSpan, action)
	}

	return results
}

/*
func toLogs(rl plog.ResourceLogs) {
	rl.SetSchemaUrl(semconv.SchemaURL)

	in := rl.ScopeLogs().AppendEmpty()
	in.Scope().SetName("Datadog")

	logRecord := in.LogRecords().AppendEmpty()
	logRecord.Attributes().PutBool("datadog.is_rum", true)
}

func toTraces(event map[string]any, tracesResource ptrace.ResourceSpans) {
	scope := tracesResource.ScopeSpans().AppendEmpty()
	scope.Scope().SetName("Datadog")

	traceID, spanID, err := parseIDs(event)
	if err != nil {
		fmt.Println(err)
		return
	}

	newSpan := scope.Spans().AppendEmpty()
	newSpan.SetName("RUMResource")
	newSpan.SetTraceID(uInt64ToTraceID(0, traceID))
	newSpan.SetSpanID(uInt64ToSpanID(spanID))
	newSpan.Attributes().PutBool("datadog.is_rum", true)

	startTime, duration, err := parseTimestamp(event)
	if err != nil {
		fmt.Println(err)
		return
	}
	newSpan.SetStartTimestamp(startTime)
	tracesResource.Resource().Attributes().PutInt("datadog.rum.duration", int64(duration))
	newSpan.SetEndTimestamp(pcommon.Timestamp(uint64(startTime) + uint64(duration)))

	// Session metrics
	newSpan.Attributes().PutInt("test", 1)
}

func DDToRumOTLP(jsonEvents []map[string]any, req *http.Request, rawRequestBody []byte) (ptrace.Traces, plog.Logs) {
	serviceGroups := make(map[string][]map[string]any)
	for _, event := range jsonEvents {
		service, ok := event["service"].(string)
		if !ok {
			fmt.Println("Event missing service name, skipping")
			continue
		}
		serviceGroups[service] = append(serviceGroups[service], event)
	}

	// sharedResource := pcommon.NewResource()
	logs := plog.NewLogs()
	traces := ptrace.NewTraces()

	for service, events := range serviceGroups {
		sharedResource := pcommon.NewResource()
		sharedResource.Attributes().PutStr("service", service)

		logsResource := logs.ResourceLogs().AppendEmpty()
		tracesResource := traces.ResourceSpans().AppendEmpty()

		for _, event := range events {
			ParseRUMRequestIntoResource(sharedResource, event, req, rawRequestBody)
			_, ok := event["_dd"].(map[string]any)["trace_id"].(string)
			if !ok {
				fmt.Println("failed to retrieve traceID from RUM event payload; treating as log instead")
				toLogs(logsResource)
			} else {
				toTraces(event, tracesResource)
			}
		}
		logsResource.Resource().CopyTo(sharedResource)
		tracesResource.Resource().CopyTo(sharedResource)
	}

	return traces, logs
	// return sharedResource
}
*/

func ParseRUMRequestIntoResource(resource pcommon.Resource, payload map[string]any, req *http.Request, rawRequestBody []byte) {
	resource.Attributes().PutStr(semconv.AttributeServiceName, "browser-rum-sdk")
	resource.Attributes().PutBool("datadog.is_rum", true)

	prettyPayload, _ := json.MarshalIndent(payload, "", "\t")
	resource.Attributes().PutStr("pretty_payload", string(prettyPayload))

	bodyDump := resource.Attributes().PutEmptyBytes("request_body_dump")
	bodyDump.FromRaw(rawRequestBody)

	// Store HTTP headers as attributes
	headerAttrs := resource.Attributes().PutEmptyMap("request_headers")
	for headerName, headerValues := range req.Header {
		headerValueList := headerAttrs.PutEmptySlice(headerName)
		for _, headerValue := range headerValues {
			headerValueList.AppendEmpty().SetStr(headerValue)
		}
	}

	// Store URL query parameters as attributes
	queryAttrs := resource.Attributes().PutEmptyMap("request_query")
	for paramName, paramValues := range req.URL.Query() {
		paramValueList := queryAttrs.PutEmptySlice(paramName)
		for _, paramValue := range paramValues {
			paramValueList.AppendEmpty().SetStr(paramValue)
		}
	}

	resource.Attributes().PutStr("request_ddforward", req.URL.Query().Get("ddforward"))
}

func uInt64ToTraceID(high, low uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[0:8], high)
	binary.BigEndian.PutUint64(traceID[8:16], low)
	return pcommon.TraceID(traceID)
}

func uInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pcommon.SpanID(spanID)
}
