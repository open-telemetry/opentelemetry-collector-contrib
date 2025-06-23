package translator

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

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

func ParseW3CTraceContext(traceparent string) (traceID pcommon.TraceID, spanID pcommon.SpanID, err error) {
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

func parseIDs(payload map[string]any, req *http.Request) (pcommon.TraceID, pcommon.SpanID, error) {
	ddMetadata, ok := payload["_dd"].(map[string]any)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to find _dd metadata in payload")
	}

	traceIDString, ok := ddMetadata["trace_id"].(string)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to retrieve traceID from payload")
	}
	traceID, err := strconv.ParseUint(traceIDString, 10, 64)
	if err != nil {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to parse traceID: %w", err)
	}

	spanIDString, ok := ddMetadata["span_id"].(string)
	if !ok {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to retrieve spanID from payload")
	}
	spanID, err := strconv.ParseUint(spanIDString, 10, 64)
	if err != nil {
		return pcommon.NewTraceIDEmpty(), pcommon.NewSpanIDEmpty(), fmt.Errorf("failed to parse spanID: %w", err)
	}

	return uInt64ToTraceID(0, traceID), uInt64ToSpanID(spanID), nil
}

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

	if timeSpent, ok := session[ATTR_TIME_SPENT].(float64); ok {
		newSpan.Attributes().PutInt(PREFIX_DATADOG+"."+PREFIX_SESSION+"."+ATTR_TIME_SPENT, int64(timeSpent))
	}

	countCategories := []string{
		PREFIX_ERROR,
		ATTR_LONG_TASK,
		PREFIX_RESOURCE,
		PREFIX_ACTION,
		ATTR_FRUSTRATION,
	}
	for _, category := range countCategories {
		if countCategory, ok := session[category].(map[string]any); ok {
			setNumberAttributes(newSpan, countCategory, PREFIX_DATADOG+"."+PREFIX_SESSION+"."+category+".", []string{ATTR_COUNT})
		}
	}

	if id, ok := session[ATTR_ID].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_SESSION+"."+ATTR_ID, id)
	}
	if ip, ok := session[ATTR_IP].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_CLIENT+"."+ATTR_ADDRESS, ip)
	}

	if isActive, ok := session[ATTR_IS_ACTIVE].(bool); ok {
		newSpan.Attributes().PutBool(PREFIX_DATADOG+"."+PREFIX_SESSION+"."+ATTR_IS_ACTIVE, isActive)
	}

	sessionAttributes := []string{
		ATTR_TYPE,
		ATTR_REFERRER,
	}
	setStringAttributes(newSpan, session, PREFIX_DATADOG+"."+PREFIX_SESSION+".", sessionAttributes)

	if initialView, ok := session[PREFIX_INITIAL_VIEW].(map[string]any); ok {
		initialViewAttributes := []string{
			ATTR_ID,
			ATTR_URL_HOST,
			ATTR_URL_PATH,
			ATTR_URL_PATH_GROUP,
		}
		setStringAttributes(newSpan, initialView, PREFIX_DATADOG+"."+PREFIX_SESSION+"."+PREFIX_INITIAL_VIEW+".", initialViewAttributes)

		if urlQuery, ok := initialView[ATTR_URL_QUERY].(map[string]any); ok {
			queryMap := newSpan.Attributes().PutEmptyMap(PREFIX_DATADOG + "." + PREFIX_SESSION + "." + PREFIX_INITIAL_VIEW + "." + ATTR_URL_QUERY)
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := initialView[ATTR_URL_SCHEME].(map[string]any); ok {
			schemeMap := newSpan.Attributes().PutEmptyMap(PREFIX_DATADOG + "." + PREFIX_SESSION + "." + PREFIX_INITIAL_VIEW + "." + ATTR_URL_SCHEME)
			for key, value := range urlScheme {
				if strValue, ok := value.(string); ok {
					schemeMap.PutStr(key, strValue)
				}
			}
		}
	}

	if lastView, ok := session[PREFIX_LAST_VIEW].(map[string]any); ok {
		lastViewAttributes := []string{
			ATTR_ID,
			ATTR_URL_HOST,
			ATTR_URL_PATH,
			ATTR_URL_PATH_GROUP,
		}
		setStringAttributes(newSpan, lastView, PREFIX_DATADOG+"."+PREFIX_SESSION+"."+PREFIX_LAST_VIEW+".", lastViewAttributes)

		if urlQuery, ok := lastView[ATTR_URL_QUERY].(map[string]any); ok {
			queryMap := newSpan.Attributes().PutEmptyMap(PREFIX_DATADOG + "." + PREFIX_SESSION + "." + PREFIX_LAST_VIEW + "." + ATTR_URL_QUERY)
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := lastView[ATTR_URL_SCHEME].(map[string]any); ok {
			schemeMap := newSpan.Attributes().PutEmptyMap(PREFIX_DATADOG + "." + PREFIX_SESSION + "." + PREFIX_LAST_VIEW + "." + ATTR_URL_SCHEME)
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
		ATTR_TIME_SPENT,
		ATTR_FIRST_BYTE,
		ATTR_LARGEST_CONTENTFUL_PAINT,
		ATTR_FIRST_INPUT_DELAY,
		ATTR_INTERACTION_TO_NEXT_PAINT,
		ATTR_CUMULATIVE_LAYOUT_SHIFT,
		ATTR_LOADING_TIME,
		ATTR_FIRST_CONTENTFUL_PAINT,
		ATTR_DOM_INTERACTIVE,
		ATTR_DOM_CONTENT_LOADED,
		ATTR_DOM_COMPLETE,
		ATTR_LOAD_EVENT,
	}
	setNumberAttributes(newSpan, view, PREFIX_DATADOG+"."+PREFIX_VIEW+".", viewMetrics)

	countCategories := []string{
		PREFIX_ERROR,
		ATTR_LONG_TASK,
		PREFIX_RESOURCE,
		PREFIX_ACTION,
		ATTR_FRUSTRATION,
	}
	for _, category := range countCategories {
		if countCategory, ok := view[category].(map[string]any); ok {
			setNumberAttributes(newSpan, countCategory, PREFIX_DATADOG+"."+PREFIX_VIEW+"."+category+".", []string{ATTR_COUNT})
		}
	}
	//TO-DO: create mappings doc on confluence
	viewAttributes := []string{
		ATTR_ID,
		ATTR_LOADING_TYPE,
		ATTR_REFERRER,
		ATTR_URL,
		ATTR_URL_HASH,
		ATTR_URL_HOST,
		ATTR_URL_PATH,
		ATTR_URL_PATH_GROUP,
		ATTR_LARGEST_CONTENTFUL_PAINT_TARGET_SELECTOR,
		ATTR_FIRST_INPUT_DELAY_TARGET_SELECTOR,
		ATTR_INTERACTION_TO_NEXT_PAINT_TARGET_SELECTOR,
		ATTR_CUMULATIVE_LAYOUT_SHIFT_TARGET_SELECTOR,
	}
	setStringAttributes(newSpan, view, PREFIX_DATADOG+"."+PREFIX_VIEW+".", viewAttributes)

	if urlQuery, ok := view[ATTR_URL_QUERY].(map[string]any); ok {
		urlQueryAttributes := []string{
			ATTR_UTM_SOURCE,
			ATTR_UTM_MEDIUM,
			ATTR_UTM_CAMPAIGN,
			ATTR_UTM_CONTENT,
			ATTR_UTM_TERM,
		}
		setStringAttributes(newSpan, urlQuery, PREFIX_DATADOG+"."+PREFIX_VIEW+"."+ATTR_URL_QUERY+".", urlQueryAttributes)
	}

	if urlScheme, ok := view[ATTR_URL_SCHEME].(map[string]any); ok {
		schemeMap := newSpan.Attributes().PutEmptyMap(PREFIX_DATADOG + "." + PREFIX_VIEW + "." + ATTR_URL_SCHEME)
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
		ATTR_DURATION,
		ATTR_SIZE,
		ATTR_STATUS_CODE,
	}
	setNumberAttributes(newSpan, resource, PREFIX_DATADOG+"."+PREFIX_RESOURCE+".", resourceMetrics)

	durationCategories := []string{
		ATTR_SSL,
		ATTR_DNS,
		ATTR_REDIRECT,
		ATTR_FIRST_BYTE,
		ATTR_DOWNLOAD,
	}
	for _, category := range durationCategories {
		if durationCategory, ok := resource[category].(map[string]any); ok {
			setNumberAttributes(newSpan, durationCategory, PREFIX_DATADOG+"."+PREFIX_RESOURCE+"."+category+".", []string{ATTR_DURATION})
		}
	}

	resourceAttributes := []string{
		ATTR_TYPE,
		ATTR_METHOD,
		ATTR_URL,
		ATTR_URL_HOST,
		ATTR_URL_PATH,
		ATTR_URL_SCHEME,
	}
	setStringAttributes(newSpan, resource, PREFIX_DATADOG+"."+PREFIX_RESOURCE+".", resourceAttributes)

	if urlQuery, ok := resource[ATTR_URL_QUERY].(map[string]any); ok {
		queryMap := newSpan.Attributes().PutEmptyMap(PREFIX_DATADOG + "." + PREFIX_RESOURCE + "." + ATTR_URL_QUERY)
		for key, value := range urlQuery {
			if strValue, ok := value.(string); ok {
				queryMap.PutStr(key, strValue)
			}
		}
	}

	if provider, ok := resource[ATTR_PROVIDER].(map[string]any); ok {
		providerAttributes := []string{
			ATTR_NAME,
			ATTR_DOMAIN,
			ATTR_TYPE,
		}
		setStringAttributes(newSpan, provider, PREFIX_DATADOG+"."+PREFIX_RESOURCE+"."+ATTR_PROVIDER+".", providerAttributes)
	}
}

func setError(newSpan ptrace.Span, error map[string]any) {
	if error == nil {
		return
	}

	ddErrorAttributes := []string{
		ATTR_SOURCE,
		ATTR_STACK,
	}
	setStringAttributes(newSpan, error, PREFIX_DATADOG+"."+PREFIX_ERROR+".", ddErrorAttributes)

	errorAttributes := []string{
		ATTR_TYPE,
		ATTR_MESSAGE,
	}
	setStringAttributes(newSpan, error, PREFIX_ERROR+".", errorAttributes)
}

func setAction(newSpan ptrace.Span, action map[string]any) {
	if action == nil {
		return
	}

	if loadingTime, ok := action[ATTR_LOADING_TIME].(float64); ok {
		newSpan.Attributes().PutInt(PREFIX_DATADOG+"."+PREFIX_ACTION+"."+ATTR_LOADING_TIME, int64(loadingTime))
	}

	countCategories := []string{
		PREFIX_ERROR,
		ATTR_LONG_TASK,
		PREFIX_RESOURCE,
	}
	for _, category := range countCategories {
		if countCategory, ok := action[category].(map[string]any); ok {
			setNumberAttributes(newSpan, countCategory, PREFIX_DATADOG+"."+PREFIX_ACTION+"."+category+".", []string{"count"})
		}
	}

	actionAttributes := []string{
		ATTR_ID,
		ATTR_TYPE,
		ATTR_NAME,
	}
	setStringAttributes(newSpan, action, PREFIX_DATADOG+"."+PREFIX_ACTION+".", actionAttributes)

	if target, ok := action[ATTR_TARGET].(map[string]any); ok {
		setStringAttributes(newSpan, target, PREFIX_DATADOG+"."+PREFIX_ACTION+"."+ATTR_TARGET+".", []string{"name"})
	}

	if frustration, ok := action[ATTR_FRUSTRATION].(map[string]any); ok {
		frustrationAttributes := []string{
			ATTR_TYPE + ":" + ATTR_DEAD_CLICK,
			ATTR_TYPE + ":" + ATTR_RAGE_CLICK,
			ATTR_TYPE + ":" + ATTR_ERROR_CLICK,
		}
		setStringAttributes(newSpan, frustration, PREFIX_DATADOG+"."+PREFIX_ACTION+"."+ATTR_FRUSTRATION+".", frustrationAttributes)
	}
}

func setGeo(newSpan ptrace.Span, geo map[string]any) {
	if geo == nil {
		return
	}

	if countryIsoCode, ok := geo[ATTR_COUNTRY_ISO_CODE].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_GEO+"."+ATTR_COUNTRY+"."+ATTR_ISO_CODE, countryIsoCode)
	}

	if city, ok := geo[ATTR_CITY].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_GEO+"."+ATTR_LOCALITY+"."+ATTR_NAME, city)
	}

	if continentCode, ok := geo[ATTR_CONTINENT_CODE].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_GEO+"."+ATTR_CONTINENT_CODE, continentCode)
	}

	geoAttributes := []string{
		ATTR_COUNTRY,
		ATTR_COUNTRY_SUBDIVISION,
		ATTR_CONTINENT,
	}
	setStringAttributes(newSpan, geo, PREFIX_DATADOG+"."+PREFIX_GEO+".", geoAttributes)
}

func ToLogs(payload map[string]any, req *http.Request, reqBytes []byte) plog.Logs {
	results := plog.NewLogs()
	rl := results.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl(semconv.SchemaURL)
	ParseRUMRequestIntoResource(rl.Resource(), payload, req, reqBytes)

	in := rl.ScopeLogs().AppendEmpty()
	in.Scope().SetName("Datadog")

	newLogRecord := in.LogRecords().AppendEmpty()
	newLogRecord.Attributes().PutBool(PREFIX_DATADOG+"."+ATTR_IS_RUM, true)

	fmt.Println("%%%%% SUCCESSFUL LOG PARSE")
	return results
}

func ToTraces(payload map[string]any, req *http.Request, reqBytes []byte, traceparent string) ptrace.Traces {
	results := ptrace.NewTraces()
	rs := results.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl(semconv.SchemaURL)
	ParseRUMRequestIntoResource(rs.Resource(), payload, req, reqBytes)

	in := rs.ScopeSpans().AppendEmpty()
	in.Scope().SetName("Datadog")

	traceID, spanID, err := ParseW3CTraceContext(traceparent)
	fmt.Println("%%%%%%%%%%%%%% W3C TRACE ID: ", traceID)
	fmt.Println("%%%%%%%%%%%%%% W3C SPAN ID: ", spanID)
	if err != nil {
		err = nil
		traceID, spanID, err = parseIDs(payload, req)
		if err != nil {
			fmt.Println(err)
			return results
		}
	}

	fmt.Println("%%%%%%%%%%%%%% TRACE ID: ", traceID)
	fmt.Println("%%%%%%%%%%%%%% SPAN ID: ", spanID)
	newSpan := in.Spans().AppendEmpty()
	newSpan.SetName("RUMResource")
	newSpan.SetTraceID(traceID)
	newSpan.SetSpanID(spanID)
	newSpan.Attributes().PutBool(PREFIX_DATADOG+"."+ATTR_IS_RUM, true)

	if date, ok := payload[ATTR_DATE].(float64); ok {
		newSpan.Attributes().PutInt(PREFIX_DATADOG+"."+ATTR_DATE, int64(date))
	}

	if device, ok := payload[PREFIX_DEVICE].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_DATADOG+"."+PREFIX_DEVICE, device)
	}

	if service, ok := payload[ATTR_SERVICE].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_DATADOG+"."+ATTR_SERVICE, service)
	}

	if user, ok := payload[PREFIX_USR].(map[string]any); ok {
		userAttributes := []string{
			ATTR_ID,
			ATTR_NAME,
			ATTR_EMAIL,
		}
		setStringAttributes(newSpan, user, PREFIX_USER+".", userAttributes)
	}

	if eventType, ok := payload[ATTR_TYPE].(string); ok {
		newSpan.Attributes().PutStr(PREFIX_DATADOG+"."+ATTR_TYPE, eventType)
	}

	if application, ok := payload[PREFIX_APPLICATION].(map[string]any); ok {
		applicationAttributes := []string{
			ATTR_ID,
			ATTR_NAME,
		}
		setStringAttributes(newSpan, application, PREFIX_DATADOG+"."+PREFIX_APPLICATION+".", applicationAttributes)
	}

	//TO-DO: device.type and device.name
	if device, ok := payload[PREFIX_DEVICE].(map[string]any); ok {
		if deviceType, ok := device[ATTR_TYPE].(string); ok {
			newSpan.Attributes().PutStr(PREFIX_DATADOG+"."+PREFIX_DEVICE+"."+ATTR_TYPE, deviceType)
		}
		if brand, ok := device[ATTR_BRAND].(string); ok {
			newSpan.Attributes().PutStr(PREFIX_DEVICE+"."+ATTR_MANUFACTURER, brand)
		}
		if model, ok := device[ATTR_MODEL].(string); ok {
			newSpan.Attributes().PutStr(PREFIX_DEVICE+"."+PREFIX_MODEL+"."+ATTR_IDENTIFIER, model)
		}
		if name, ok := device[ATTR_NAME].(string); ok {
			newSpan.Attributes().PutStr(PREFIX_DEVICE+"."+PREFIX_MODEL+"."+ATTR_NAME, name)
		}
	}

	if os, ok := payload[PREFIX_OS].(map[string]any); ok {
		osAttributes := []string{
			ATTR_NAME,
			ATTR_VERSION,
		}
		setStringAttributes(newSpan, os, PREFIX_OS, osAttributes)

		setStringAttributes(newSpan, os, PREFIX_DATADOG+"."+PREFIX_OS+".", []string{ATTR_VERSION_MAJOR})
	}

	if geo, ok := payload[PREFIX_GEO].(map[string]any); ok {
		setGeo(newSpan, geo)
	}

	if session, ok := payload[PREFIX_SESSION].(map[string]any); ok {
		setSession(newSpan, session)
	}

	if view, ok := payload[PREFIX_VIEW].(map[string]any); ok {
		setView(newSpan, view)
	}

	if resource, ok := payload[PREFIX_RESOURCE].(map[string]any); ok {
		setResource(newSpan, resource)
	}

	if longTask, ok := payload[ATTR_LONG_TASK].(map[string]any); ok {
		if duration, ok := longTask[ATTR_DURATION].(float64); ok {
			newSpan.Attributes().PutInt(PREFIX_DATADOG+"."+ATTR_LONG_TASK+"."+ATTR_DURATION, int64(duration))
		}
	}

	if error, ok := payload[PREFIX_ERROR].(map[string]any); ok {
		setError(newSpan, error)
	}

	if action, ok := payload[PREFIX_ACTION].(map[string]any); ok {
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
