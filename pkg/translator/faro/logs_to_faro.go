// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-logfmt/logfmt"
	faroTypes "github.com/grafana/faro/pkg/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/multierr"
)

const (
	faroKind          = "kind"
	faroTimestamp     = "timestamp"
	faroContextPrefix = "context_"

	faroSDKName         = "sdk_name"
	faroSDKVersion      = "sdk_version"
	faroSDKIntegrations = "sdk_integrations"

	faroApp            = "app"
	faroAppName        = "app_name"
	faroAppNamespace   = "app_namespace"
	faroAppRelease     = "app_release"
	faroAppVersion     = "app_version"
	faroAppBundleID    = "app_bundle_id"
	faroAppEnvironment = "app_environment"

	faroBrowserName           = "browser_name"
	faroBrowserVersion        = "browser_version"
	faroBrowserOS             = "browser_os"
	faroBrowserMobile         = "browser_mobile"
	faroBrowserLanguage       = "browser_language"
	faroBrowserUserAgent      = "browser_userAgent"
	faroBrowserViewportHeight = "browser_viewportHeight"
	faroBrowserViewportWidth  = "browser_viewportWidth"
	faroBrowserBrands         = "browser_brands"
	faroBrowserBrandPrefix    = "browser_brand_"

	faroBrand        = "brand"
	faroBrandVersion = "version"

	faroGeoContinentIso   = "geo_continent_iso"
	faroGeoCountryIso     = "geo_country_iso"
	faroGeoSubdivisionIso = "geo_subdivision_iso"
	faroGeoCity           = "geo_city"
	faroGeoASNOrg         = "geo_asn_org"
	faroGeoASNID          = "geo_asn_id"

	faroIsK6Browser = "k6_isK6Browser"

	faroPageID         = "page_id"
	faroPageURL        = "page_url"
	faroPageAttrPrefix = "page_attr_"

	faroSessionID         = "session_id"
	faroSessionAttrPrefix = "session_attr_"

	faroUserID         = "user_id"
	faroUserEmail      = "user_email"
	faroUsername       = "user_username"
	faroUserAttrPrefix = "user_attr_"

	faroViewName = "view_name"

	faroLogMessage = "message"
	faroLogLevel   = "level"

	faroTraceID = "traceID"
	faroSpanID  = "spanID"

	faroEventDomain     = "event_domain"
	faroEventName       = "event_name"
	faroEventDataPrefix = "event_data_"

	faroExceptionType       = "type"
	faroExceptionValue      = "value"
	faroExceptionHash       = "hash"
	faroExceptionStacktrace = "stacktrace"

	faroStacktraceFunction = "function"
	faroStackTraceModule   = "module"
	faroStackTraceFilename = "filename"
	faroStackTraceLineno   = "lineno"
	faroStackTraceColno    = "colno"

	faroMeasurementType        = "type"
	faroMeasurementValuePrefix = "value_"

	faroActionID       = "action_id"
	faroActionName     = "action_name"
	faroActionParentID = "action_parent_id"
)

var stacktraceRegexp *regexp.Regexp

func init() {
	stacktraceRegexp = regexp.MustCompile(`(?P<function>.+)?\s\(((?P<module>.+)\|)?(?P<filename>.+)?:(?P<lineno>\d+)?:(?P<colno>\d+)?\)$`)
}

// TranslateFromLogs converts a Logs pipeline data into []*faro.Payload
func TranslateFromLogs(ctx context.Context, ld plog.Logs) ([]faroTypes.Payload, error) {
	_, span := otel.Tracer("").Start(ctx, "TranslateFromLogs")
	defer span.End()

	metaMap := make(map[string]*faroTypes.Payload)
	var payloads []faroTypes.Payload
	rls := ld.ResourceLogs()

	w := sha256.New()
	encoder := json.NewEncoder(w)
	var errs error
	for i := 0; i < rls.Len(); i++ {
		scopeLogs := rls.At(i).ScopeLogs()
		resource := rls.At(i).Resource()
		for j := 0; j < scopeLogs.Len(); j++ {
			logs := scopeLogs.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				payload, err := translateLogToFaroPayload(log, resource)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}

				meta := payload.Meta
				w.Reset() // Reset hash before encoding
				if encodeErr := encoder.Encode(meta); encodeErr != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				metaKey := fmt.Sprintf("%x", w.Sum(nil))
				// if payload meta already exists in the metaMap merge payload to the existing payload
				existingPayload, found := metaMap[metaKey]
				if found {
					mergePayloads(existingPayload, payload)
				} else {
					metaMap[metaKey] = &payload
				}
			}
		}
	}

	if len(metaMap) == 0 {
		return payloads, errs
	}

	payloads = make([]faroTypes.Payload, 0)
	for _, payload := range metaMap {
		payloads = append(payloads, *payload)
	}

	span.SetAttributes(attribute.Int("count", len(payloads)))
	return payloads, errs
}

func mergePayloads(target *faroTypes.Payload, source faroTypes.Payload) {
	// merge logs
	target.Logs = append(target.Logs, source.Logs...)

	// merge events
	target.Events = append(target.Events, source.Events...)

	// merge measurements
	target.Measurements = append(target.Measurements, source.Measurements...)

	// merge exceptions
	target.Exceptions = append(target.Exceptions, source.Exceptions...)

	// merge traces
	if source.Traces != nil {
		sourceTraces := ptrace.NewTraces()
		source.Traces.CopyTo(sourceTraces)
		if target.Traces == nil {
			target.Traces = &faroTypes.Traces{
				Traces: ptrace.NewTraces(),
			}
		}
		sourceTraces.ResourceSpans().MoveAndAppendTo(target.Traces.ResourceSpans())
	}
}

func translateLogToFaroPayload(lr plog.LogRecord, rl pcommon.Resource) (faroTypes.Payload, error) {
	var payload faroTypes.Payload
	body := lr.Body().Str()
	kv, err := parseLogfmtLine(body)
	if err != nil {
		return payload, err
	}
	kind, ok := kv[faroKind]
	if !ok {
		return payload, fmt.Errorf("%s log record body doesn't contain kind", body)
	}

	var errs error

	switch kind {
	case string(faroTypes.KindLog):
		payload, err = convertLogKeyValToPayload(kv)
	case string(faroTypes.KindEvent):
		payload, err = convertEventKeyValsToPayload(kv)
	case string(faroTypes.KindMeasurement):
		payload, err = convertMeasurementKeyValsToPayload(kv)
	case string(faroTypes.KindException):
		payload, err = convertExceptionKeyValsToPayload(kv)
	default:
		return payload, fmt.Errorf("kind: %s is not supported, it must be one of %s, %s, %s, %s", kind, faroTypes.KindLog, faroTypes.KindEvent, faroTypes.KindMeasurement, faroTypes.KindException)
	}
	if err != nil {
		errs = multierr.Append(errs, err)
	}
	meta, metaErr := extractMetaFromKeyVal(kv, rl)
	if metaErr != nil {
		return payload, multierr.Append(errs, metaErr)
	}
	payload.Meta = meta
	return payload, errs
}

func parseLogfmtLine(line string) (map[string]string, error) {
	kv := make(map[string]string)
	d := logfmt.NewDecoder(strings.NewReader(line))
	for d.ScanRecord() {
		for d.ScanKeyval() {
			key := string(d.Key())
			value := string(d.Value())
			kv[key] = value
		}
	}
	if err := d.Err(); err != nil {
		return nil, err
	}
	return kv, nil
}

func convertLogKeyValToPayload(kv map[string]string) (faroTypes.Payload, error) {
	var payload faroTypes.Payload
	log, err := extractLogFromKeyVal(kv)
	if err != nil {
		return payload, err
	}
	payload.Logs = []faroTypes.Log{
		log,
	}
	return payload, nil
}

func convertEventKeyValsToPayload(kv map[string]string) (faroTypes.Payload, error) {
	var payload faroTypes.Payload
	event, err := extractEventFromKeyVal(kv)
	if err != nil {
		return payload, err
	}
	payload.Events = []faroTypes.Event{
		event,
	}
	return payload, nil
}

func convertExceptionKeyValsToPayload(kv map[string]string) (faroTypes.Payload, error) {
	var payload faroTypes.Payload
	exception, err := extractExceptionFromKeyVal(kv)
	if err != nil {
		return payload, err
	}
	payload.Exceptions = []faroTypes.Exception{
		exception,
	}
	return payload, nil
}

func convertMeasurementKeyValsToPayload(kv map[string]string) (faroTypes.Payload, error) {
	var payload faroTypes.Payload
	measurement, err := extractMeasurementFromKeyVal(kv)
	if err != nil {
		return payload, err
	}
	payload.Measurements = []faroTypes.Measurement{
		measurement,
	}
	return payload, nil
}

func extractMetaFromKeyVal(kv map[string]string, rl pcommon.Resource) (faroTypes.Meta, error) {
	var meta faroTypes.Meta
	app := extractAppFromKeyVal(kv, rl)
	browser, err := extractBrowserFromKeyVal(kv)
	if err != nil {
		return meta, err
	}
	geo := extractGeoFromKeyVal(kv)
	k6, err := extractK6FromKeyVal(kv)
	if err != nil {
		return meta, err
	}
	page := extractPageFromKeyVal(kv)
	sdk := extractSDKFromKeyVal(kv)
	session := extractSessionFromKeyVal(kv)
	user := extractUserFromKeyVal(kv)
	view := extractViewFromKeyVal(kv)

	meta.App = app
	meta.Browser = *browser
	meta.Geo = geo
	meta.K6 = k6
	meta.Page = page
	meta.SDK = sdk
	meta.Session = session
	meta.User = user
	meta.View = view
	return meta, nil
}

func extractTimestampFromKeyVal(kv map[string]string) (time.Time, error) {
	var timestamp time.Time
	val, ok := kv[faroTimestamp]
	if !ok {
		return timestamp, nil
	}
	timestamp, err := time.Parse(string(faroTypes.TimeFormatRFC3339Milli), val)
	if err != nil {
		return timestamp, err
	}
	return timestamp, nil
}

func extractSDKFromKeyVal(kv map[string]string) faroTypes.SDK {
	var sdk faroTypes.SDK
	if name, ok := kv[faroSDKName]; ok {
		sdk.Name = name
	}
	if version, ok := kv[faroSDKVersion]; ok {
		sdk.Version = version
	}
	if integrationsStr, ok := kv[faroSDKIntegrations]; ok {
		sdk.Integrations = parseIntegrationsFromString(integrationsStr)
	}
	return sdk
}

func parseIntegrationsFromString(integrationsString string) []faroTypes.SDKIntegration {
	sdkIntegrations := make([]faroTypes.SDKIntegration, 0)
	if len(integrationsString) == 0 {
		return sdkIntegrations
	}

	for _, integrationString := range strings.Split(integrationsString, ",") {
		integrationNameVersion := strings.Split(integrationString, ":")
		sdkIntegrations = append(sdkIntegrations, faroTypes.SDKIntegration{
			Name:    integrationNameVersion[0],
			Version: integrationNameVersion[1],
		})
	}

	return sdkIntegrations
}

func extractAppFromKeyVal(kv map[string]string, rl pcommon.Resource) faroTypes.App {
	var app faroTypes.App
	rl.Attributes().Range(func(key string, val pcommon.Value) bool {
		if key == string(semconv.ServiceNameKey) {
			app.Name = val.Str()
		}
		if key == string(semconv.ServiceNamespaceKey) {
			app.Namespace = val.Str()
		}
		if key == string(semconv.ServiceVersionKey) {
			app.Version = val.Str()
		}
		if key == string(semconv.DeploymentEnvironmentKey) {
			app.Environment = val.Str()
		}
		// force the app name stored in resource attribute service.name
		// if service.name resource attribute is missing try to get app name from the custom "app" resource attribute
		if key == faroApp && app.Name == "" {
			app.Name = val.Str()
		}
		if key == faroAppBundleID {
			app.BundleID = val.Str()
		}
		return true
	})
	if name, ok := kv[faroAppName]; ok {
		// force the app name stored in resource attribute service.name or in custom "app" resource attribute
		// if service.name resource attribute is missing as well as custom "app" attribute try to get app name from the log line
		if app.Name == "" {
			app.Name = name
		}
	}
	if namespace, ok := kv[faroAppNamespace]; ok {
		// force the app namespace stored in resource attribute service.namespace
		// if service.namespace resource attribute is missing try to get app namespace from the log line
		if app.Namespace == "" {
			app.Namespace = namespace
		}
	}
	if release, ok := kv[faroAppRelease]; ok {
		app.Release = release
	}
	if version, ok := kv[faroAppVersion]; ok {
		// force the app version stored in resource attribute service.version
		// if service.version resource attribute is missing try to get app version from the log line
		if app.Version == "" {
			app.Version = version
		}
	}
	if environment, ok := kv[faroAppEnvironment]; ok {
		// force the app environment stored in resource attribute deployment.environment
		// if deployment.environment resource attribute is missing try to get app environment from the log line
		if app.Environment == "" {
			app.Environment = environment
		}
	}
	return app
}

func extractBrowserFromKeyVal(kv map[string]string) (*faroTypes.Browser, error) {
	var browser faroTypes.Browser
	if name, ok := kv[faroBrowserName]; ok {
		browser.Name = name
	}
	if version, ok := kv[faroBrowserVersion]; ok {
		browser.Version = version
	}
	if os, ok := kv[faroBrowserOS]; ok {
		browser.OS = os
	}
	if mobile, ok := kv[faroBrowserMobile]; ok {
		isMobile, err := strconv.ParseBool(mobile)
		if err != nil {
			return nil, err
		}
		browser.Mobile = isMobile
	}
	if language, ok := kv[faroBrowserLanguage]; ok {
		browser.Language = language
	}
	if userAgent, ok := kv[faroBrowserUserAgent]; ok {
		browser.UserAgent = userAgent
	}
	if viewportHeight, ok := kv[faroBrowserViewportHeight]; ok {
		browser.ViewportHeight = viewportHeight
	}
	if viewportWidth, ok := kv[faroBrowserViewportWidth]; ok {
		browser.ViewportWidth = viewportWidth
	}

	browserBrands, err := extractBrowserBrandsFromKeyVal(kv)
	if err != nil {
		return nil, err
	}
	browser.Brands = browserBrands
	return &browser, nil
}

func extractBrowserBrandsFromKeyVal(kv map[string]string) (faroTypes.Browser_Brands, error) {
	var brands faroTypes.Browser_Brands
	if brandsAsString, ok := kv[faroBrowserBrands]; ok {
		if err := brands.FromBrandsString(brandsAsString); err != nil {
			return brands, err
		}
		return brands, nil
	}

	brandsMap := make(map[int64]faroTypes.Brand)
	for key, val := range kv {
		if suffix, found := strings.CutPrefix(key, faroBrowserBrandPrefix); found {
			brandAsString := strings.Split(suffix, "_")
			idx, err := strconv.ParseInt(brandAsString[0], 10, 64)
			if err != nil {
				return brands, err
			}
			brand, ok := brandsMap[idx]
			if !ok {
				brandsMap[idx] = faroTypes.Brand{}
				brand = brandsMap[idx]
			}
			if brandAsString[1] == faroBrand {
				brand.Brand = val
			}
			if brandAsString[1] == faroBrandVersion {
				brand.Version = val
			}
			brandsMap[idx] = brand
		}
	}
	brandsMapLen := len(brandsMap)
	if brandsMapLen != 0 {
		brandsAsArray := make([]faroTypes.Brand, brandsMapLen)
		for i, brand := range brandsMap {
			brandsAsArray[i] = brand
		}
		if err := brands.FromBrandsArray(brandsAsArray); err != nil {
			return brands, err
		}
	}
	return brands, nil
}

func extractGeoFromKeyVal(kv map[string]string) faroTypes.Geo {
	var geo faroTypes.Geo
	if continentISOCode, ok := kv[faroGeoContinentIso]; ok {
		geo.ContinentISOCode = continentISOCode
	}
	if countryISOCode, ok := kv[faroGeoCountryIso]; ok {
		geo.CountryISOCode = countryISOCode
	}
	if subdivisionISO, ok := kv[faroGeoSubdivisionIso]; ok {
		geo.SubdivisionISO = subdivisionISO
	}
	if city, ok := kv[faroGeoCity]; ok {
		geo.City = city
	}
	if asnOrg, ok := kv[faroGeoASNOrg]; ok {
		geo.ASNOrg = asnOrg
	}
	if asnID, ok := kv[faroGeoASNID]; ok {
		geo.ASNID = asnID
	}

	return geo
}

func extractK6FromKeyVal(kv map[string]string) (faroTypes.K6, error) {
	var k6 faroTypes.K6
	if isK6BrowserStr, ok := kv[faroIsK6Browser]; ok {
		isK6Browser, err := strconv.ParseBool(isK6BrowserStr)
		if err != nil {
			return k6, err
		}
		k6.IsK6Browser = isK6Browser
	}
	return k6, nil
}

func extractPageFromKeyVal(kv map[string]string) faroTypes.Page {
	var page faroTypes.Page
	if id, ok := kv[faroPageID]; ok {
		page.ID = id
	}
	if url, ok := kv[faroPageURL]; ok {
		page.URL = url
	}
	page.Attributes = extractAttributesWithPrefixFromKeyVal(faroPageAttrPrefix, kv)
	return page
}

func extractSessionFromKeyVal(kv map[string]string) faroTypes.Session {
	var session faroTypes.Session
	if id, ok := kv[faroSessionID]; ok {
		session.ID = id
	}
	session.Attributes = extractAttributesWithPrefixFromKeyVal(faroSessionAttrPrefix, kv)
	return session
}

func extractUserFromKeyVal(kv map[string]string) faroTypes.User {
	var user faroTypes.User
	if email, ok := kv[faroUserEmail]; ok {
		user.Email = email
	}
	if id, ok := kv[faroUserID]; ok {
		user.ID = id
	}
	if username, ok := kv[faroUsername]; ok {
		user.Username = username
	}

	user.Attributes = extractAttributesWithPrefixFromKeyVal(faroUserAttrPrefix, kv)
	return user
}

func extractViewFromKeyVal(kv map[string]string) faroTypes.View {
	var view faroTypes.View
	if name, ok := kv[faroViewName]; ok {
		view.Name = name
	}
	return view
}

func extractLogFromKeyVal(kv map[string]string) (faroTypes.Log, error) {
	var log faroTypes.Log
	timestamp, err := extractTimestampFromKeyVal(kv)
	if err != nil {
		return log, err
	}
	log.Timestamp = timestamp

	if message, ok := kv[faroLogMessage]; ok {
		log.Message = message
	}

	if level, ok := kv[faroLogLevel]; ok {
		switch level {
		case string(faroTypes.LogLevelError):
			log.LogLevel = faroTypes.LogLevelError
		case string(faroTypes.LogLevelWarning):
			log.LogLevel = faroTypes.LogLevelWarning
		case string(faroTypes.LogLevelTrace):
			log.LogLevel = faroTypes.LogLevelTrace
		case string(faroTypes.LogLevelInfo):
			log.LogLevel = faroTypes.LogLevelInfo
		case string(faroTypes.LogLevelDebug):
			log.LogLevel = faroTypes.LogLevelDebug
		}
	}

	logContext := extractLogContextFromKeyVal(kv)
	if len(logContext) > 0 {
		log.Context = logContext
	}
	trace := extractTraceFromKeyVal(kv)
	log.Trace = trace
	log.Action = extractActionFromKeyVal(kv)
	return log, nil
}

func extractLogContextFromKeyVal(kv map[string]string) faroTypes.LogContext {
	logContext := make(faroTypes.LogContext, 0)
	for key, val := range kv {
		if after, found := strings.CutPrefix(key, faroContextPrefix); found {
			logContext[after] = val
		}
	}

	return logContext
}

func extractTraceFromKeyVal(kv map[string]string) faroTypes.TraceContext {
	var trace faroTypes.TraceContext
	if traceID, ok := kv[faroTraceID]; ok {
		trace.TraceID = traceID
	}
	if spanID, ok := kv[faroSpanID]; ok {
		trace.SpanID = spanID
	}
	return trace
}

func extractActionFromKeyVal(kv map[string]string) faroTypes.Action {
	var action faroTypes.Action
	if name, ok := kv[faroActionName]; ok {
		action.Name = name
	}
	if parentID, ok := kv[faroActionParentID]; ok {
		action.ParentID = parentID
	}
	if id, ok := kv[faroActionID]; ok {
		action.ID = id
	}
	return action
}

func extractEventFromKeyVal(kv map[string]string) (faroTypes.Event, error) {
	var event faroTypes.Event
	if domain, ok := kv[faroEventDomain]; ok {
		event.Domain = domain
	}
	if name, ok := kv[faroEventName]; ok {
		event.Name = name
	}
	timestamp, err := extractTimestampFromKeyVal(kv)
	if err != nil {
		return event, err
	}
	event.Timestamp = timestamp
	trace := extractTraceFromKeyVal(kv)
	event.Trace = trace
	event.Attributes = extractAttributesWithPrefixFromKeyVal(faroEventDataPrefix, kv)
	event.Action = extractActionFromKeyVal(kv)
	return event, nil
}

func extractExceptionFromKeyVal(kv map[string]string) (faroTypes.Exception, error) {
	var exception faroTypes.Exception
	if exceptionType, ok := kv[faroExceptionType]; ok {
		exception.Type = exceptionType
	}
	if exceptionValue, ok := kv[faroExceptionValue]; ok {
		exception.Value = exceptionValue
	}
	exceptionContext := extractExceptionContextFromKeyVal(kv)
	if len(exceptionContext) > 0 {
		exception.Context = exceptionContext
	}
	stacktrace, err := extractStacktraceFromKeyVal(kv, exception.Type, exception.Value)
	if err != nil {
		return exception, err
	}
	exception.Stacktrace = stacktrace
	timestamp, err := extractTimestampFromKeyVal(kv)
	if err != nil {
		return exception, err
	}
	exception.Timestamp = timestamp
	trace := extractTraceFromKeyVal(kv)
	exception.Trace = trace
	exception.Action = extractActionFromKeyVal(kv)
	return exception, nil
}

func extractExceptionContextFromKeyVal(kv map[string]string) faroTypes.ExceptionContext {
	exceptionContext := make(faroTypes.ExceptionContext, 0)
	for key, val := range kv {
		if after, found := strings.CutPrefix(key, faroContextPrefix); found {
			exceptionContext[after] = val
		}
	}

	return exceptionContext
}

func extractStacktraceFromKeyVal(kv map[string]string, exceptionType string, exceptionValue string) (*faroTypes.Stacktrace, error) {
	stacktraceStr, ok := kv[faroExceptionStacktrace]
	if !ok {
		return nil, nil
	}
	stacktrace, err := parseStacktraceFromString(stacktraceStr, exceptionType, exceptionValue)
	if err != nil {
		return nil, err
	}
	return stacktrace, nil
}

func parseStacktraceFromString(stacktraceStr, exceptionType, exceptionValue string) (*faroTypes.Stacktrace, error) {
	var stacktrace faroTypes.Stacktrace
	frames := make([]faroTypes.Frame, 0)
	framesAsString, _ := strings.CutPrefix(stacktraceStr, fmt.Sprintf("%s: %s", exceptionType, exceptionValue))
	framesAsArrayOfStrings := strings.Split(framesAsString, "\n  at ")
	for _, frameStr := range framesAsArrayOfStrings {
		frame, err := parseFrameFromString(frameStr)
		if err != nil {
			return nil, err
		}
		if frame != nil {
			frames = append(frames, *frame)
		}
	}
	if len(frames) > 0 {
		stacktrace.Frames = frames
	}
	return &stacktrace, nil
}

func parseFrameFromString(frameStr string) (*faroTypes.Frame, error) {
	var frame faroTypes.Frame
	if len(frameStr) == 0 {
		return nil, nil
	}

	matches := stacktraceRegexp.FindStringSubmatch(frameStr)

	frame.Function = matches[stacktraceRegexp.SubexpIndex(faroStacktraceFunction)]
	frame.Module = matches[stacktraceRegexp.SubexpIndex(faroStackTraceModule)]
	frame.Filename = matches[stacktraceRegexp.SubexpIndex(faroStackTraceFilename)]

	if linenoStr := matches[stacktraceRegexp.SubexpIndex(faroStackTraceLineno)]; linenoStr != "" {
		lineno, err := strconv.ParseInt(linenoStr, 10, 64)
		if err != nil {
			return nil, err
		}
		frame.Lineno = int(lineno)
	}
	if colnoStr := matches[stacktraceRegexp.SubexpIndex(faroStackTraceColno)]; colnoStr != "" {
		colno, err := strconv.ParseInt(colnoStr, 10, 64)
		if err != nil {
			return nil, err
		}
		frame.Colno = int(colno)
	}

	return &frame, nil
}

func extractMeasurementFromKeyVal(kv map[string]string) (faroTypes.Measurement, error) {
	var measurement faroTypes.Measurement
	if measurementType, ok := kv[faroMeasurementType]; ok {
		measurement.Type = measurementType
	}
	measurementContext := extractMeasurementContextFromKeyVal(kv)
	if len(measurementContext) > 0 {
		measurement.Context = measurementContext
	}
	timestamp, err := extractTimestampFromKeyVal(kv)
	if err != nil {
		return measurement, err
	}
	measurement.Timestamp = timestamp
	trace := extractTraceFromKeyVal(kv)
	measurement.Trace = trace
	measurementValues, err := extractMeasurementValuesFromKeyVal(kv)
	if err != nil {
		return measurement, err
	}
	measurement.Values = measurementValues
	measurement.Action = extractActionFromKeyVal(kv)
	return measurement, nil
}

func extractMeasurementContextFromKeyVal(kv map[string]string) faroTypes.MeasurementContext {
	measurementContext := make(faroTypes.MeasurementContext, 0)
	for key, val := range kv {
		if after, found := strings.CutPrefix(key, faroContextPrefix); found {
			measurementContext[after] = val
		}
	}

	return measurementContext
}

func extractMeasurementValuesFromKeyVal(kv map[string]string) (map[string]float64, error) {
	values := make(map[string]float64, 0)
	for key, val := range kv {
		if valueName, found := strings.CutPrefix(key, faroMeasurementValuePrefix); found {
			valFloat64, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return nil, err
			}
			values[valueName] = valFloat64
		}
	}
	if len(values) > 0 {
		return values, nil
	}
	return nil, nil
}

func extractAttributesWithPrefixFromKeyVal(prefix string, kv map[string]string) map[string]string {
	attributes := make(map[string]string, 0)
	for key, val := range kv {
		if attrName, found := strings.CutPrefix(key, prefix); found {
			attributes[attrName] = val
		}
	}
	if len(attributes) > 0 {
		return attributes
	}
	return nil
}
