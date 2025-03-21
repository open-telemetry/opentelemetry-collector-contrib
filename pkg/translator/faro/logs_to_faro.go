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
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/multierr"
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
				// if payload meta already exists in the metaMap merge payload to the existing payload
				if encodeErr := encoder.Encode(meta); encodeErr != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				metaKey := fmt.Sprintf("%x", w.Sum(nil))
				w.Reset()
				existingPayload, found := metaMap[metaKey]
				if found {
					// merge payloads with the same meta
					mergePayloads(existingPayload, payload)
				} else {
					// if payload meta doesn't exist in the metaMap add new meta key to the metaMap
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
		sourceTraces.ResourceSpans().MoveAndAppendTo(target.Traces.Traces.ResourceSpans())
	}
}

func translateLogToFaroPayload(lr plog.LogRecord, rl pcommon.Resource) (faroTypes.Payload, error) {
	var payload faroTypes.Payload
	body := lr.Body().Str()
	kv, err := parseLogfmtLine(body)
	if err != nil {
		return payload, err
	}
	kind, ok := kv["kind"]
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
	val, ok := kv["timestamp"]
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
	if name, ok := kv["sdk_name"]; ok {
		sdk.Name = name
	}
	if version, ok := kv["sdk_version"]; ok {
		sdk.Version = version
	}
	if integrationsStr, ok := kv["sdk_integrations"]; ok {
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
		if key == "app" && app.Name == "" {
			app.Name = val.Str()
		}
		if key == "app_bundle_id" {
			app.BundleID = val.Str()
		}
		return true
	})
	if name, ok := kv["app_name"]; ok {
		// force the app name stored in resource attribute service.name or in custom "app" resource attribute
		// if service.name resource attribute is missing as well as custom "app" attribute try to get app name from the log line
		if app.Name == "" {
			app.Name = name
		}
	}
	if namespace, ok := kv["app_namespace"]; ok {
		// force the app namespace stored in resource attribute service.namespace
		// if service.namespace resource attribute is missing try to get app namespace from the log line
		if app.Namespace == "" {
			app.Namespace = namespace
		}
	}
	if release, ok := kv["app_release"]; ok {
		app.Release = release
	}
	if version, ok := kv["app_version"]; ok {
		// force the app version stored in resource attribute service.version
		// if service.version resource attribute is missing try to get app version from the log line
		if app.Version == "" {
			app.Version = version
		}
	}
	if environment, ok := kv["app_environment"]; ok {
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
	if name, ok := kv["browser_name"]; ok {
		browser.Name = name
	}
	if version, ok := kv["browser_version"]; ok {
		browser.Version = version
	}
	if os, ok := kv["browser_os"]; ok {
		browser.OS = os
	}
	if mobile, ok := kv["browser_mobile"]; ok {
		isMobile, err := strconv.ParseBool(mobile)
		if err != nil {
			return nil, err
		}
		browser.Mobile = isMobile
	}
	if language, ok := kv["browser_language"]; ok {
		browser.Language = language
	}
	if userAgent, ok := kv["user_agent"]; ok {
		browser.UserAgent = userAgent
	}
	if viewportHeight, ok := kv["browser_viewportHeight"]; ok {
		browser.ViewportHeight = viewportHeight
	}
	if viewportWidth, ok := kv["browser_viewportWidth"]; ok {
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
	if brandsAsString, ok := kv["browser_brands"]; ok {
		if err := brands.FromBrandsString(brandsAsString); err != nil {
			return brands, err
		}
		return brands, nil
	}

	brandsMap := make(map[int64]faroTypes.Brand)
	for key, val := range kv {
		if suffix, found := strings.CutPrefix(key, "browser_brand_"); found {
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
			if brandAsString[1] == "brand" {
				brand.Brand = val
			}
			if brandAsString[1] == "version" {
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
	if continentISOCode, ok := kv["geo_continent_iso"]; ok {
		geo.ContinentISOCode = continentISOCode
	}
	if countryISOCode, ok := kv["geo_country_iso"]; ok {
		geo.CountryISOCode = countryISOCode
	}
	if subdivisionISO, ok := kv["geo_subdivision_iso"]; ok {
		geo.SubdivisionISO = subdivisionISO
	}
	if city, ok := kv["geo_city"]; ok {
		geo.City = city
	}
	if asnOrg, ok := kv["geo_asn_org"]; ok {
		geo.ASNOrg = asnOrg
	}
	if asnID, ok := kv["geo_asn_id"]; ok {
		geo.ASNID = asnID
	}

	return geo
}

func extractK6FromKeyVal(kv map[string]string) (faroTypes.K6, error) {
	var k6 faroTypes.K6
	if isK6BrowserStr, ok := kv["k6_isK6Browser"]; ok {
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
	if id, ok := kv["page_id"]; ok {
		page.ID = id
	}
	if url, ok := kv["page_url"]; ok {
		page.URL = url
	}
	page.Attributes = extractAttributesWithPrefixFromKeyVal("page_attr_", kv)
	return page
}

func extractSessionFromKeyVal(kv map[string]string) faroTypes.Session {
	var session faroTypes.Session
	if id, ok := kv["session_id"]; ok {
		session.ID = id
	}
	session.Attributes = extractAttributesWithPrefixFromKeyVal("session_attr_", kv)
	return session
}

func extractUserFromKeyVal(kv map[string]string) faroTypes.User {
	var user faroTypes.User
	if email, ok := kv["user_email"]; ok {
		user.Email = email
	}
	if id, ok := kv["user_id"]; ok {
		user.ID = id
	}
	if username, ok := kv["user_username"]; ok {
		user.Username = username
	}

	user.Attributes = extractAttributesWithPrefixFromKeyVal("user_attr_", kv)
	return user
}

func extractViewFromKeyVal(kv map[string]string) faroTypes.View {
	var view faroTypes.View
	if name, ok := kv["view_name"]; ok {
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

	if message, ok := kv["message"]; ok {
		log.Message = message
	}

	if level, ok := kv["level"]; ok {
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
	return log, nil
}

func extractLogContextFromKeyVal(kv map[string]string) faroTypes.LogContext {
	logContext := make(faroTypes.LogContext, 0)
	for key, val := range kv {
		if after, found := strings.CutPrefix(key, "context_"); found {
			logContext[after] = val
		}
	}

	return logContext
}

func extractTraceFromKeyVal(kv map[string]string) faroTypes.TraceContext {
	var trace faroTypes.TraceContext
	if traceID, ok := kv["traceID"]; ok {
		trace.TraceID = traceID
	}
	if spanID, ok := kv["spanID"]; ok {
		trace.SpanID = spanID
	}
	return trace
}

func extractEventFromKeyVal(kv map[string]string) (faroTypes.Event, error) {
	var event faroTypes.Event
	if domain, ok := kv["event_domain"]; ok {
		event.Domain = domain
	}
	if name, ok := kv["event_name"]; ok {
		event.Name = name
	}
	timestamp, err := extractTimestampFromKeyVal(kv)
	if err != nil {
		return event, err
	}
	event.Timestamp = timestamp
	trace := extractTraceFromKeyVal(kv)
	event.Trace = trace
	event.Attributes = extractAttributesWithPrefixFromKeyVal("event_data_", kv)
	return event, nil
}

func extractExceptionFromKeyVal(kv map[string]string) (faroTypes.Exception, error) {
	var exception faroTypes.Exception
	if exceptionType, ok := kv["type"]; ok {
		exception.Type = exceptionType
	}
	if exceptionValue, ok := kv["value"]; ok {
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
	return exception, nil
}

func extractExceptionContextFromKeyVal(kv map[string]string) faroTypes.ExceptionContext {
	exceptionContext := make(faroTypes.ExceptionContext, 0)
	for key, val := range kv {
		if after, found := strings.CutPrefix(key, "context_"); found {
			exceptionContext[after] = val
		}
	}

	return exceptionContext
}

func extractStacktraceFromKeyVal(kv map[string]string, exceptionType string, exceptionValue string) (*faroTypes.Stacktrace, error) {
	stacktraceStr, ok := kv["stacktrace"]
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

	frame.Function = matches[stacktraceRegexp.SubexpIndex("function")]
	frame.Module = matches[stacktraceRegexp.SubexpIndex("module")]
	frame.Filename = matches[stacktraceRegexp.SubexpIndex("filename")]

	if linenoStr := matches[stacktraceRegexp.SubexpIndex("lineno")]; linenoStr != "" {
		lineno, err := strconv.ParseInt(linenoStr, 10, 64)
		if err != nil {
			return nil, err
		}
		frame.Lineno = int(lineno)
	}
	if colnoStr := matches[stacktraceRegexp.SubexpIndex("colno")]; colnoStr != "" {
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
	if measurementType, ok := kv["type"]; ok {
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
	return measurement, nil
}

func extractMeasurementContextFromKeyVal(kv map[string]string) faroTypes.MeasurementContext {
	measurementContext := make(faroTypes.MeasurementContext, 0)
	for key, val := range kv {
		if after, found := strings.CutPrefix(key, "context_"); found {
			measurementContext[after] = val
		}
	}

	return measurementContext
}

func extractMeasurementValuesFromKeyVal(kv map[string]string) (map[string]float64, error) {
	values := make(map[string]float64, 0)
	for key, val := range kv {
		if valueName, found := strings.CutPrefix(key, "value_"); found {
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
