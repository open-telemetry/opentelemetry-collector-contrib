package translator

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.5.0"
)

func ToLogs(payload map[string]any, req *http.Request, reqBytes []byte) plog.Logs {
	results := plog.NewLogs()
	rl := results.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl(semconv.SchemaURL)
	parseRUMRequestIntoResource(rl.Resource(), payload, req, reqBytes)

	in := rl.ScopeLogs().AppendEmpty()
	in.Scope().SetName(InstrumentationScopeName)

	newLogRecord := in.LogRecords().AppendEmpty()

	if eventType, ok := payload[AttrType].(string); ok {
		newLogRecord.Body().SetStr("datadog.rum." + eventType)
	} else {
		newLogRecord.Body().SetStr("datadog.rum.event")
	}

	if date, ok := payload[AttrDate].(float64); ok {
		newLogRecord.SetTimestamp(pcommon.Timestamp(int64(date) * 1000000)) // Convert to nanoseconds
		newLogRecord.Attributes().PutInt(PrefixDatadog+"."+AttrDate, int64(date))
	}

	if device, ok := payload[PrefixDevice].(string); ok {
		newLogRecord.Attributes().PutStr(PrefixDatadog+"."+PrefixDevice, device)
	}

	if service, ok := payload[AttrService].(string); ok {
		newLogRecord.Attributes().PutStr(PrefixDatadog+"."+AttrService, service)
	}

	if user, ok := payload[PrefixUsr].(map[string]any); ok {
		userAttributes := []string{
			AttrID,
			AttrName,
			AttrEmail,
		}
		setStringAttributesForLog(newLogRecord, user, PrefixUser+".", userAttributes)
	}

	if eventType, ok := payload[AttrType].(string); ok {
		newLogRecord.Attributes().PutStr(PrefixDatadog+"."+AttrType, eventType)
	}

	if application, ok := payload[PrefixApplication].(map[string]any); ok {
		applicationAttributes := []string{
			AttrID,
			AttrName,
		}
		setStringAttributesForLog(newLogRecord, application, PrefixDatadog+"."+PrefixApplication+".", applicationAttributes)
	}

	if device, ok := payload[PrefixDevice].(map[string]any); ok {
		if deviceType, ok := device[AttrType].(string); ok {
			newLogRecord.Attributes().PutStr(PrefixDatadog+"."+PrefixDevice+"."+AttrType, deviceType)
		}
		if brand, ok := device[AttrBrand].(string); ok {
			newLogRecord.Attributes().PutStr(PrefixDevice+"."+AttrManufacturer, brand)
		}
		if model, ok := device[AttrModel].(string); ok {
			newLogRecord.Attributes().PutStr(PrefixDevice+"."+PrefixModel+"."+AttrIdentifier, model)
		}
		if name, ok := device[AttrName].(string); ok {
			newLogRecord.Attributes().PutStr(PrefixDevice+"."+PrefixModel+"."+AttrName, name)
		}
	}

	if os, ok := payload[PrefixOS].(map[string]any); ok {
		osAttributes := []string{
			AttrName,
			AttrVersion,
		}
		setStringAttributesForLog(newLogRecord, os, PrefixOS, osAttributes)

		setStringAttributesForLog(newLogRecord, os, PrefixDatadog+"."+PrefixOS+".", []string{AttrVersionMajor})
	}

	if geo, ok := payload[PrefixGeo].(map[string]any); ok {
		setGeoForLog(newLogRecord, geo)
	}

	if session, ok := payload[PrefixSession].(map[string]any); ok {
		setSessionForLog(newLogRecord, session)
	}

	if view, ok := payload[PrefixView].(map[string]any); ok {
		setViewForLog(newLogRecord, view)
	}

	if resource, ok := payload[PrefixResource].(map[string]any); ok {
		setResourceForLog(newLogRecord, resource)
	}

	if longTask, ok := payload[AttrLongTask].(map[string]any); ok {
		if duration, ok := longTask[AttrDuration].(float64); ok {
			newLogRecord.Attributes().PutInt(PrefixDatadog+"."+AttrLongTask+"."+AttrDuration, int64(duration))
		}
	}

	if error, ok := payload[PrefixError].(map[string]any); ok {
		setErrorForLog(newLogRecord, error)
	}

	if action, ok := payload[PrefixAction].(map[string]any); ok {
		setActionForLog(newLogRecord, action)
	}

	fmt.Println("%%%%% SUCCESSFUL LOG PARSE")
	return results
}

func setNumberAttributesForLog(logRecord plog.LogRecord, data map[string]any, prefix string, attributes []string) {
	for _, attribute := range attributes {
		if value, ok := data[attribute].(float64); ok {
			logRecord.Attributes().PutInt(prefix+attribute, int64(value))
		}
	}
}

func setStringAttributesForLog(logRecord plog.LogRecord, data map[string]any, prefix string, attributes []string) {
	for _, attribute := range attributes {
		if value, ok := data[attribute].(string); ok {
			logRecord.Attributes().PutStr(prefix+attribute, value)
		}
	}
}

func setSessionForLog(logRecord plog.LogRecord, session map[string]any) {
	if session == nil {
		return
	}

	if timeSpent, ok := session[AttrTimeSpent].(float64); ok {
		logRecord.Attributes().PutInt(PrefixDatadog+"."+PrefixSession+"."+AttrTimeSpent, int64(timeSpent))
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
			setNumberAttributesForLog(logRecord, countCategory, PrefixDatadog+"."+PrefixSession+"."+category+".", []string{AttrCount})
		}
	}

	if id, ok := session[AttrID].(string); ok {
		logRecord.Attributes().PutStr(PrefixSession+"."+AttrID, id)
	}
	if ip, ok := session[AttrIP].(string); ok {
		logRecord.Attributes().PutStr(PrefixClient+"."+AttrAddress, ip)
	}

	if isActive, ok := session[AttrIsActive].(bool); ok {
		logRecord.Attributes().PutBool(PrefixDatadog+"."+PrefixSession+"."+AttrIsActive, isActive)
	}

	sessionAttributes := []string{
		AttrType,
		AttrReferrer,
	}
	setStringAttributesForLog(logRecord, session, PrefixDatadog+"."+PrefixSession+".", sessionAttributes)

	if initialView, ok := session[PrefixInitialView].(map[string]any); ok {
		initialViewAttributes := []string{
			AttrID,
			AttrURLHost,
			AttrURLPath,
			AttrURLPathGroup,
		}
		setStringAttributesForLog(logRecord, initialView, PrefixDatadog+"."+PrefixSession+"."+PrefixInitialView+".", initialViewAttributes)

		if urlQuery, ok := initialView[AttrURLQuery].(map[string]any); ok {
			queryMap := logRecord.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixInitialView + "." + AttrURLQuery)
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := initialView[AttrURLScheme].(map[string]any); ok {
			schemeMap := logRecord.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixInitialView + "." + AttrURLScheme)
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
		setStringAttributesForLog(logRecord, lastView, PrefixDatadog+"."+PrefixSession+"."+PrefixLastView+".", lastViewAttributes)

		if urlQuery, ok := lastView[AttrURLQuery].(map[string]any); ok {
			queryMap := logRecord.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixLastView + "." + AttrURLQuery)
			for key, value := range urlQuery {
				if strValue, ok := value.(string); ok {
					queryMap.PutStr(key, strValue)
				}
			}
		}

		if urlScheme, ok := lastView[AttrURLScheme].(map[string]any); ok {
			schemeMap := logRecord.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixSession + "." + PrefixLastView + "." + AttrURLScheme)
			for key, value := range urlScheme {
				if strValue, ok := value.(string); ok {
					schemeMap.PutStr(key, strValue)
				}
			}
		}
	}
}

func setViewForLog(logRecord plog.LogRecord, view map[string]any) {
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
	setNumberAttributesForLog(logRecord, view, PrefixDatadog+"."+PrefixView+".", viewMetrics)

	countCategories := []string{
		PrefixError,
		AttrLongTask,
		PrefixResource,
		PrefixAction,
		AttrFrustration,
	}
	for _, category := range countCategories {
		if countCategory, ok := view[category].(map[string]any); ok {
			setNumberAttributesForLog(logRecord, countCategory, PrefixDatadog+"."+PrefixView+"."+category+".", []string{AttrCount})
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
	setStringAttributesForLog(logRecord, view, PrefixDatadog+"."+PrefixView+".", viewAttributes)

	if urlQuery, ok := view[AttrURLQuery].(map[string]any); ok {
		urlQueryAttributes := []string{
			AttrUTMSource,
			AttrUTMMedium,
			AttrUTMCampaign,
			AttrUTMContent,
			AttrUTMTerm,
		}
		setStringAttributesForLog(logRecord, urlQuery, PrefixDatadog+"."+PrefixView+"."+AttrURLQuery+".", urlQueryAttributes)
	}

	if urlScheme, ok := view[AttrURLScheme].(map[string]any); ok {
		schemeMap := logRecord.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixView + "." + AttrURLScheme)
		for key, value := range urlScheme {
			if strValue, ok := value.(string); ok {
				schemeMap.PutStr(key, strValue)
			}
		}
	}
}

func setResourceForLog(logRecord plog.LogRecord, resource map[string]any) {
	if resource == nil {
		return
	}

	resourceMetrics := []string{
		AttrDuration,
		AttrSize,
		AttrStatusCode,
	}
	setNumberAttributesForLog(logRecord, resource, PrefixDatadog+"."+PrefixResource+".", resourceMetrics)

	durationCategories := []string{
		AttrSSL,
		AttrDNS,
		AttrRedirect,
		AttrFirstByte,
		AttrDownload,
	}
	for _, category := range durationCategories {
		if durationCategory, ok := resource[category].(map[string]any); ok {
			setNumberAttributesForLog(logRecord, durationCategory, PrefixDatadog+"."+PrefixResource+"."+category+".", []string{AttrDuration})
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
	setStringAttributesForLog(logRecord, resource, PrefixDatadog+"."+PrefixResource+".", resourceAttributes)

	if urlQuery, ok := resource[AttrURLQuery].(map[string]any); ok {
		queryMap := logRecord.Attributes().PutEmptyMap(PrefixDatadog + "." + PrefixResource + "." + AttrURLQuery)
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
		setStringAttributesForLog(logRecord, provider, PrefixDatadog+"."+PrefixResource+"."+AttrProvider+".", providerAttributes)
	}
}

func setErrorForLog(logRecord plog.LogRecord, error map[string]any) {
	if error == nil {
		return
	}

	ddErrorAttributes := []string{
		AttrSource,
		AttrStack,
	}
	setStringAttributesForLog(logRecord, error, PrefixDatadog+"."+PrefixError+".", ddErrorAttributes)

	errorAttributes := []string{
		AttrType,
		AttrMessage,
	}
	setStringAttributesForLog(logRecord, error, PrefixError+".", errorAttributes)
}

func setActionForLog(logRecord plog.LogRecord, action map[string]any) {
	if action == nil {
		return
	}

	if loadingTime, ok := action[AttrLoadingTime].(float64); ok {
		logRecord.Attributes().PutInt(PrefixDatadog+"."+PrefixAction+"."+AttrLoadingTime, int64(loadingTime))
	}

	countCategories := []string{
		PrefixError,
		AttrLongTask,
		PrefixResource,
	}
	for _, category := range countCategories {
		if countCategory, ok := action[category].(map[string]any); ok {
			setNumberAttributesForLog(logRecord, countCategory, PrefixDatadog+"."+PrefixAction+"."+category+".", []string{"count"})
		}
	}

	actionAttributes := []string{
		AttrID,
		AttrType,
		AttrName,
	}
	setStringAttributesForLog(logRecord, action, PrefixDatadog+"."+PrefixAction+".", actionAttributes)

	if target, ok := action[AttrTarget].(map[string]any); ok {
		setStringAttributesForLog(logRecord, target, PrefixDatadog+"."+PrefixAction+"."+AttrTarget+".", []string{"name"})
	}

	if frustration, ok := action[AttrFrustration].(map[string]any); ok {
		frustrationAttributes := []string{
			AttrType + ":" + AttrDeadClick,
			AttrType + ":" + AttrRageClick,
			AttrType + ":" + AttrErrorClick,
		}
		setStringAttributesForLog(logRecord, frustration, PrefixDatadog+"."+PrefixAction+"."+AttrFrustration+".", frustrationAttributes)
	}
}

func setGeoForLog(logRecord plog.LogRecord, geo map[string]any) {
	if geo == nil {
		return
	}

	if countryIsoCode, ok := geo[AttrCountryISOCode].(string); ok {
		logRecord.Attributes().PutStr(PrefixGeo+"."+AttrCountry+"."+AttrISOCode, countryIsoCode)
	}

	if city, ok := geo[AttrCity].(string); ok {
		logRecord.Attributes().PutStr(PrefixGeo+"."+AttrLocality+"."+AttrName, city)
	}

	if continentCode, ok := geo[AttrContinentCode].(string); ok {
		logRecord.Attributes().PutStr(PrefixGeo+"."+AttrContinentCode, continentCode)
	}

	geoAttributes := []string{
		AttrCountry,
		AttrCountrySubdivision,
		AttrContinent,
	}
	setStringAttributesForLog(logRecord, geo, PrefixDatadog+"."+PrefixGeo+".", geoAttributes)
} 