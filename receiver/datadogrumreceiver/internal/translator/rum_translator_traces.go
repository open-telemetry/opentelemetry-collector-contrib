package translator

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

func ToTraces(payload map[string]any, req *http.Request, reqBytes []byte, traceparent string) ptrace.Traces {
	results := ptrace.NewTraces()
	rs := results.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl(semconv.SchemaURL)
	parseRUMRequestIntoResource(rs.Resource(), payload, req, reqBytes)

	in := rs.ScopeSpans().AppendEmpty()
	in.Scope().SetName(InstrumentationScopeName)

	traceID, spanID, err := parseW3CTraceContext(traceparent)
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
	if eventType, ok := payload[AttrType].(string); ok {
		newSpan.SetName("datadog.rum." + eventType)
	} else {
		newSpan.SetName("datadog.rum.event")
	}
	newSpan.SetTraceID(traceID)
	newSpan.SetSpanID(spanID)

	if date, ok := payload[AttrDate].(float64); ok {
		newSpan.Attributes().PutInt(PrefixDatadog+"."+AttrDate, int64(date))
	}

	if device, ok := payload[PrefixDevice].(string); ok {
		newSpan.Attributes().PutStr(PrefixDatadog+"."+PrefixDevice, device)
	}

	if service, ok := payload[AttrService].(string); ok {
		newSpan.Attributes().PutStr(PrefixDatadog+"."+AttrService, service)
	}

	if user, ok := payload[PrefixUsr].(map[string]any); ok {
		userAttributes := []string{
			AttrID,
			AttrName,
			AttrEmail,
		}
		setStringAttributesForTrace(newSpan, user, PrefixUser+".", userAttributes)
	}

	if eventType, ok := payload[AttrType].(string); ok {
		newSpan.Attributes().PutStr(PrefixDatadog+"."+AttrType, eventType)
	}

	if application, ok := payload[PrefixApplication].(map[string]any); ok {
		applicationAttributes := []string{
			AttrID,
			AttrName,
		}
		setStringAttributesForTrace(newSpan, application, PrefixDatadog+"."+PrefixApplication+".", applicationAttributes)
	}

	if device, ok := payload[PrefixDevice].(map[string]any); ok {
		if deviceType, ok := device[AttrType].(string); ok {
			newSpan.Attributes().PutStr(PrefixDatadog+"."+PrefixDevice+"."+AttrType, deviceType)
		}
		if brand, ok := device[AttrBrand].(string); ok {
			newSpan.Attributes().PutStr(PrefixDevice+"."+AttrManufacturer, brand)
		}
		if model, ok := device[AttrModel].(string); ok {
			newSpan.Attributes().PutStr(PrefixDevice+"."+PrefixModel+"."+AttrIdentifier, model)
		}
		if name, ok := device[AttrName].(string); ok {
			newSpan.Attributes().PutStr(PrefixDevice+"."+PrefixModel+"."+AttrName, name)
		}
	}

	if os, ok := payload[PrefixOS].(map[string]any); ok {
		osAttributes := []string{
			AttrName,
			AttrVersion,
		}
		setStringAttributesForTrace(newSpan, os, PrefixOS, osAttributes)

		setStringAttributesForTrace(newSpan, os, PrefixDatadog+"."+PrefixOS+".", []string{AttrVersionMajor})
	}

	if geo, ok := payload[PrefixGeo].(map[string]any); ok {
		setGeoForTrace(newSpan, geo)
	}

	if session, ok := payload[PrefixSession].(map[string]any); ok {
		setSessionForTrace(newSpan, session)
	}

	if view, ok := payload[PrefixView].(map[string]any); ok {
		setViewForTrace(newSpan, view)
	}

	if resource, ok := payload[PrefixResource].(map[string]any); ok {
		setResourceForTrace(newSpan, resource)
	}

	if longTask, ok := payload[AttrLongTask].(map[string]any); ok {
		if duration, ok := longTask[AttrDuration].(float64); ok {
			newSpan.Attributes().PutInt(PrefixDatadog+"."+AttrLongTask+"."+AttrDuration, int64(duration))
		}
	}

	if error, ok := payload[PrefixError].(map[string]any); ok {
		setErrorForTrace(newSpan, error)
	}

	if action, ok := payload[PrefixAction].(map[string]any); ok {
		setActionForTrace(newSpan, action)
	}

	return results
}

func setNumberAttributesForTrace(span ptrace.Span, data map[string]any, prefix string, attributes []string) {
	for _, attribute := range attributes {
		if value, ok := data[attribute].(float64); ok {
			span.Attributes().PutInt(prefix+attribute, int64(value))
		}
	}
}

func setStringAttributesForTrace(span ptrace.Span, data map[string]any, prefix string, attributes []string) {
	for _, attribute := range attributes {
		if value, ok := data[attribute].(string); ok {
			span.Attributes().PutStr(prefix+attribute, value)
		}
	}
}

func setSessionForTrace(newSpan ptrace.Span, session map[string]any) {
	if session == nil {
		return
	}

	if timeSpent, ok := session[AttrTimeSpent].(float64); ok {
		newSpan.Attributes().PutInt(PrefixDatadog+"."+PrefixSession+"."+AttrTimeSpent, int64(timeSpent))
	}

	countCategories := []string{
		PrefixError,
		AttrLongTask,
		PrefixResource,
		PrefixAction,
		AttrFrustration,
	}
	for _, category := range countCategories {
		if countCategory, ok := session[category].(map[string]any); ok {
			setNumberAttributesForTrace(newSpan, countCategory, PrefixDatadog+"."+PrefixSession+"."+category+".", []string{AttrCount})
		}
	}

	if id, ok := session[AttrID].(string); ok {
		newSpan.Attributes().PutStr(PrefixSession+"."+AttrID, id)
	}
	if ip, ok := session[AttrIP].(string); ok {
		newSpan.Attributes().PutStr(PrefixClient+"."+AttrAddress, ip)
	}

	if isActive, ok := session[AttrIsActive].(bool); ok {
		newSpan.Attributes().PutBool(PrefixDatadog+"."+PrefixSession+"."+AttrIsActive, isActive)
	}

	sessionAttributes := []string{
		AttrType,
		AttrReferrer,
	}
	setStringAttributesForTrace(newSpan, session, PrefixDatadog+"."+PrefixSession+".", sessionAttributes)

	if initialView, ok := session[PrefixInitialView].(map[string]any); ok {
		initialViewAttributes := []string{
			AttrID,
			AttrURLHost,
			AttrURLPath,
			AttrURLPathGroup,
		}
		setStringAttributesForTrace(newSpan, initialView, PrefixDatadog+"."+PrefixSession+"."+PrefixInitialView+".", initialViewAttributes)

		if urlQuery, ok := initialView[AttrURLQuery].(map[string]any); ok {
			queryMap := newSpan.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixInitialView + "." + AttrURLQuery)
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := initialView[AttrURLScheme].(map[string]any); ok {
			schemeMap := newSpan.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixInitialView + "." + AttrURLScheme)
			for key, value := range urlScheme {
				if strValue, ok := value.(string); ok {
					schemeMap.PutStr(key, strValue)
				}
			}
		}
	}

	if lastView, ok := session[PrefixLastView].(map[string]any); ok {
		lastViewAttributes := []string{
			AttrID,
			AttrURLHost,
			AttrURLPath,
			AttrURLPathGroup,
		}
		setStringAttributesForTrace(newSpan, lastView, PrefixDatadog+"."+PrefixSession+"."+PrefixLastView+".", lastViewAttributes)

		if urlQuery, ok := lastView[AttrURLQuery].(map[string]any); ok {
			queryMap := newSpan.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixLastView + "." + AttrURLQuery)
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := lastView[AttrURLScheme].(map[string]any); ok {
			schemeMap := newSpan.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixLastView + "." + AttrURLScheme)
			for key, value := range urlScheme {
				if strValue, ok := value.(string); ok {
					schemeMap.PutStr(key, strValue)
				}
			}
		}
	}
}

func setViewForTrace(newSpan ptrace.Span, view map[string]any) {
	if view == nil {
		return
	}

	viewMetrics := []string{
		AttrTimeSpent,
		AttrFirstByte,
		AttrLargestContentfulPaint,
		AttrFirstInputDelay,
		AttrInteractionToNextPaint,
		AttrCumulativeLayoutShift,
		AttrLoadingTime,
		AttrFirstContentfulPaint,
		AttrDOMInteractive,
		AttrDOMContentLoaded,
		AttrDOMComplete,
		AttrLoadEvent,
	}
	setNumberAttributesForTrace(newSpan, view, PrefixDatadog+"."+PrefixView+".", viewMetrics)

	countCategories := []string{
		PrefixError,
		AttrLongTask,
		PrefixResource,
		PrefixAction,
		AttrFrustration,
	}
	for _, category := range countCategories {
		if countCategory, ok := view[category].(map[string]any); ok {
			setNumberAttributesForTrace(newSpan, countCategory, PrefixDatadog+"."+PrefixView+"."+category+".", []string{AttrCount})
		}
	}

	viewAttributes := []string{
		AttrID,
		AttrLoadingType,
		AttrReferrer,
		AttrURL,
		AttrURLHash,
		AttrURLHost,
		AttrURLPath,
		AttrURLPathGroup,
		AttrLargestContentfulPaintTargetSelector,
		AttrFirstInputDelayTargetSelector,
		AttrInteractionToNextPaintTargetSelector,
		AttrCumulativeLayoutShiftTargetSelector,
	}
	setStringAttributesForTrace(newSpan, view, PrefixDatadog+"."+PrefixView+".", viewAttributes)

	if urlQuery, ok := view[AttrURLQuery].(map[string]any); ok {
		urlQueryAttributes := []string{
			AttrUTMSource,
			AttrUTMMedium,
			AttrUTMCampaign,
			AttrUTMContent,
			AttrUTMTerm,
		}
		setStringAttributesForTrace(newSpan, urlQuery, PrefixDatadog+"."+PrefixView+"."+AttrURLQuery+".", urlQueryAttributes)
	}

	if urlScheme, ok := view[AttrURLScheme].(map[string]any); ok {
		schemeMap := newSpan.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixView + "." + AttrURLScheme)
		for key, value := range urlScheme {
			if strValue, ok := value.(string); ok {
				schemeMap.PutStr(key, strValue)
			}
		}
	}
}

func setResourceForTrace(newSpan ptrace.Span, resource map[string]any) {
	if resource == nil {
		return
	}

	resourceMetrics := []string{
		AttrDuration,
		AttrSize,
		AttrStatusCode,
	}
	setNumberAttributesForTrace(newSpan, resource, PrefixDatadog+"."+PrefixResource+".", resourceMetrics)

	durationCategories := []string{
		AttrSSL,
		AttrDNS,
		AttrRedirect,
		AttrFirstByte,
		AttrDownload,
	}
	for _, category := range durationCategories {
		if durationCategory, ok := resource[category].(map[string]any); ok {
			setNumberAttributesForTrace(newSpan, durationCategory, PrefixDatadog+"."+PrefixResource+"."+category+".", []string{AttrDuration})
		}
	}

	resourceAttributes := []string{
		AttrType,
		AttrMethod,
		AttrURL,
		AttrURLHost,
		AttrURLPath,
		AttrURLScheme,
	}
	setStringAttributesForTrace(newSpan, resource, PrefixDatadog+"."+PrefixResource+".", resourceAttributes)

	if urlQuery, ok := resource[AttrURLQuery].(map[string]any); ok {
		queryMap := newSpan.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixResource + "." + AttrURLQuery)
		for key, value := range urlQuery {
			if strValue, ok := value.(string); ok {
				queryMap.PutStr(key, strValue)
			}
		}
	}

	if provider, ok := resource[AttrProvider].(map[string]any); ok {
		providerAttributes := []string{
			AttrName,
			AttrDomain,
			AttrType,
		}
		setStringAttributesForTrace(newSpan, provider, PrefixDatadog+"."+PrefixResource+"."+AttrProvider+".", providerAttributes)
	}
}

func setErrorForTrace(newSpan ptrace.Span, error map[string]any) {
	if error == nil {
		return
	}

	ddErrorAttributes := []string{
		AttrSource,
		AttrStack,
	}
	setStringAttributesForTrace(newSpan, error, PrefixDatadog+"."+PrefixError+".", ddErrorAttributes)

	errorAttributes := []string{
		AttrType,
		AttrMessage,
	}
	setStringAttributesForTrace(newSpan, error, PrefixError+".", errorAttributes)
}

func setActionForTrace(newSpan ptrace.Span, action map[string]any) {
	if action == nil {
		return
	}

	if loadingTime, ok := action[AttrLoadingTime].(float64); ok {
		newSpan.Attributes().PutInt(PrefixDatadog+"."+PrefixAction+"."+AttrLoadingTime, int64(loadingTime))
	}

	countCategories := []string{
		PrefixError,
		AttrLongTask,
		PrefixResource,
	}
	for _, category := range countCategories {
		if countCategory, ok := action[category].(map[string]any); ok {
			setNumberAttributesForTrace(newSpan, countCategory, PrefixDatadog+"."+PrefixAction+"."+category+".", []string{"count"})
		}
	}

	actionAttributes := []string{
		AttrID,
		AttrType,
		AttrName,
	}
	setStringAttributesForTrace(newSpan, action, PrefixDatadog+"."+PrefixAction+".", actionAttributes)

	if target, ok := action[AttrTarget].(map[string]any); ok {
		setStringAttributesForTrace(newSpan, target, PrefixDatadog+"."+PrefixAction+"."+AttrTarget+".", []string{"name"})
	}

	if frustration, ok := action[AttrFrustration].(map[string]any); ok {
		frustrationAttributes := []string{
			AttrType + ":" + AttrDeadClick,
			AttrType + ":" + AttrRageClick,
			AttrType + ":" + AttrErrorClick,
		}
		setStringAttributesForTrace(newSpan, frustration, PrefixDatadog+"."+PrefixAction+"."+AttrFrustration+".", frustrationAttributes)
	}
}

func setGeoForTrace(newSpan ptrace.Span, geo map[string]any) {
	if geo == nil {
		return
	}

	if countryIsoCode, ok := geo[AttrCountryISOCode].(string); ok {
		newSpan.Attributes().PutStr(PrefixGeo+"."+AttrCountry+"."+AttrISOCode, countryIsoCode)
	}

	if city, ok := geo[AttrCity].(string); ok {
		newSpan.Attributes().PutStr(PrefixGeo+"."+AttrLocality+"."+AttrName, city)
	}

	if continentCode, ok := geo[AttrContinentCode].(string); ok {
		newSpan.Attributes().PutStr(PrefixGeo+"."+AttrContinentCode, continentCode)
	}

	geoAttributes := []string{
		AttrCountry,
		AttrCountrySubdivision,
		AttrContinent,
	}
	setStringAttributesForTrace(newSpan, geo, PrefixDatadog+"."+PrefixGeo+".", geoAttributes)
} 