// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure"

import (
	"bytes"
	"encoding/hex"
	"net/url"

	json "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"
)

const (
	// Constants for OpenTelemetry Specs
	traceAzureResourceID = "azure.resource.id"
)

type azureTracesRecords struct {
	Records []azureTracesRecord `json:"records"`
}

// Azure Trace Records based on Azure AppRequests & AppDependencies table data
// Fields are converted to match OpenTelemetry HTTP semantic conventions
// Refer https://opentelemetry.io/docs/specs/semconv/http/http-spans/
// the common record schema reference:
// https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/apprequests
// https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appdependencies
type azureTracesRecord struct {
	// Common fields across both AppRequests and AppDependencies tables
	Time                  string             `json:"time"`
	ResourceID            string             `json:"resourceId"`
	ResourceGUID          string             `json:"ResourceGUID"`
	Type                  string             `json:"Type"`
	AppRoleInstance       string             `json:"AppRoleInstance"`
	AppRoleName           string             `json:"AppRoleName"`
	AppVersion            string             `json:"AppVersion"`
	ClientCity            string             `json:"ClientCity"`
	ClientCountryOrRegion string             `json:"ClientCountryOrRegion"`
	ClientIP              string             `json:"ClientIP"`
	ClientStateOrProvince string             `json:"ClientStateOrProvince"`
	ClientType            string             `json:"ClientType"`
	IKey                  string             `json:"IKey"`
	OperationName         string             `json:"OperationName"`
	OperationID           string             `json:"OperationId"`
	ParentID              string             `json:"ParentId"`
	SDKVersion            string             `json:"SDKVersion"`
	Properties            map[string]string  `json:"Properties"`
	Measurements          map[string]float64 `json:"Measurements"`
	SpanID                string             `json:"Id"`
	Name                  string             `json:"Name"`
	Success               bool               `json:"Success"`
	ResultCode            string             `json:"ResultCode"`
	DurationMs            float64            `json:"DurationMs"`
	PerformanceBucket     string             `json:"PerformanceBucket"`
	ItemCount             float64            `json:"ItemCount"`
	// Fields specific to AppRequests
	URL    string `json:"Url"`
	Source string `json:"Source"`
	// Fields specific to AppDependencies
	Target         string `json:"Target"`
	DependencyType string `json:"DependencyType"`
	Data           string `json:"Data"`
}

var _ ptrace.Unmarshaler = (*TracesUnmarshaler)(nil)

type TracesUnmarshaler struct {
	Version     string
	Logger      *zap.Logger
	TimeFormats []string
}

func (r TracesUnmarshaler) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	t := ptrace.NewTraces()

	var azureTraces azureTracesRecords
	decoder := json.NewDecoder(bytes.NewReader(buf))
	err := decoder.Decode(&azureTraces)
	if err != nil {
		return t, err
	}

	resourceTraces := t.ResourceSpans().AppendEmpty()
	resource := resourceTraces.Resource()
	resource.Attributes().PutStr(string(conventions.TelemetrySDKNameKey), scopeName)
	resource.Attributes().PutStr(string(conventions.TelemetrySDKLanguageKey), conventions.TelemetrySDKLanguageGo.Value.AsString())
	resource.Attributes().PutStr(string(conventions.TelemetrySDKVersionKey), r.Version)
	resource.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	scopeSpans := resourceTraces.ScopeSpans().AppendEmpty()

	spans := scopeSpans.Spans()

	resourceID := ""
	for i := range azureTraces.Records {
		azureTrace := &azureTraces.Records[i]
		if resourceID == "" && azureTrace.ResourceID != "" {
			resourceID = azureTrace.ResourceID
		}

		resource.Attributes().PutStr("service.name", azureTrace.AppRoleName)

		nanos, err := asTimestamp(azureTrace.Time, r.TimeFormats...)
		if err != nil {
			r.Logger.Warn("Invalid Timestamp", zap.String("time", azureTrace.Time))
			continue
		}

		traceID, traceErr := TraceIDFromHex(azureTrace.OperationID)
		if traceErr != nil {
			r.Logger.Warn("Invalid TraceID", zap.String("traceID", azureTrace.OperationID))
			return t, traceErr
		}
		spanID, spanErr := SpanIDFromHex(azureTrace.SpanID)
		if spanErr != nil {
			r.Logger.Warn("Invalid SpanID", zap.String("spanID", azureTrace.SpanID))
			return t, spanErr
		}
		parentID, parentErr := SpanIDFromHex(azureTrace.ParentID)
		if parentErr != nil {
			r.Logger.Warn("Invalid ParentID so setting to nil", zap.String("parentID", azureTrace.ParentID))
		}

		span := spans.AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(spanID)
		span.SetParentSpanID(parentID)

		// Common attributes for both AppRequests and AppDependencies
		span.SetName(azureTrace.Name)
		span.Attributes().PutStr("operation.name", azureTrace.OperationName)
		span.Attributes().PutStr("cloud.app.name", azureTrace.AppRoleName)
		span.Attributes().PutStr("cloud.app.instance", azureTrace.AppRoleInstance)
		span.Attributes().PutStr("cloud.span.type", azureTrace.Type)
		span.Attributes().PutStr("azure.client.sdk.version", azureTrace.SDKVersion)
		switch azureTrace.Success {
		case true:
			span.Status().SetCode(ptrace.StatusCodeOk)
		case false:
			span.Status().SetCode(ptrace.StatusCodeError)
		default:
			span.Status().SetCode(ptrace.StatusCodeUnset)
		}
		span.Attributes().PutStr(string(conventions.UserAgentOriginalKey), azureTrace.Properties["Request-User-Agent"])

		switch azureTrace.Type {
		case "AppRequests":
			span.SetKind(ptrace.SpanKindServer)

			urlObj, _ := url.Parse(azureTrace.URL)
			hostname := urlObj.Host
			hostpath := urlObj.Path
			scheme := urlObj.Scheme

			span.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), azureTrace.Properties["HTTP Method"])
			span.Attributes().PutStr(string(conventions.ServerAddressKey), hostname)
			span.Attributes().PutStr(string(conventions.URLPathKey), hostpath)
			span.Attributes().PutStr(string(conventions.URLSchemeKey), scheme)
			span.Attributes().PutStr(string(conventions.ClientAddressKey), azureTrace.ClientIP)
			span.Attributes().PutStr("client.geo.locality.name", azureTrace.ClientCity)
			span.Attributes().PutStr("client.type", azureTrace.ClientType)
			span.Attributes().PutStr("client.geo.region.iso_code", azureTrace.ClientStateOrProvince)
			span.Attributes().PutStr("client.geo.country.iso_code", azureTrace.ClientCountryOrRegion)
			span.Attributes().PutStr(string(conventions.HTTPResponseStatusCodeKey), azureTrace.ResultCode)
			span.Attributes().PutDouble(string(conventions.HTTPResponseBodySizeKey), azureTrace.Measurements["Response Size"])
			span.Attributes().PutDouble(string(conventions.HTTPRequestBodySizeKey), azureTrace.Measurements["Request Size"])
			span.Attributes().PutStr(string(conventions.HTTPRouteKey), hostpath)
			span.Attributes().PutStr(string(conventions.URLFullKey), azureTrace.URL)

		case "AppDependencies":
			span.SetKind(ptrace.SpanKindClient)

			switch azureTrace.DependencyType {
			case "Backend":
				span.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), azureTrace.Properties["Backend Method"])
			case "HTTP":
				span.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), azureTrace.Properties["HTTP Method"])
			}

			urlObj, _ := url.Parse(azureTrace.Target)
			hostname := urlObj.Host
			scheme := urlObj.Scheme

			var hostpath string
			if azureTrace.Name != "" {
				parts := bytes.Fields([]byte(azureTrace.Name))
				if len(parts) > 1 {
					hostpath = string(parts[1])
				}
			}

			span.Attributes().PutStr(string(conventions.ServerAddressKey), hostname)
			span.Attributes().PutStr(string(conventions.URLSchemeKey), scheme)
			span.Attributes().PutStr(string(conventions.URLFullKey), azureTrace.Target)
			span.Attributes().PutStr(string(conventions.URLPathKey), hostpath)

			for key, value := range azureTrace.Properties {
				// HTTP Method and Backend Method are already mapped to http.request.method
				// Request-User-Agent is already mapped to user_agent.original
				if key != "HTTP Method" && key != "Backend Method" && key != "Request-User-Agent" {
					span.Attributes().PutStr(key, value)
				}
			}
			span.SetStartTimestamp(nanos)
			span.SetEndTimestamp(nanos + pcommon.Timestamp(azureTrace.DurationMs*1e6))
		}
	}

	if resourceID != "" {
		resourceTraces.Resource().Attributes().PutStr(traceAzureResourceID, resourceID)
	} else {
		r.Logger.Warn("No ResourceID Set on Traces!")
	}

	return t, nil
}

func TraceIDFromHex(hexStr string) (pcommon.TraceID, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return pcommon.TraceID{}, err
	}
	var id pcommon.TraceID
	copy(id[:], bytes)
	return id, nil
}

func SpanIDFromHex(hexStr string) (pcommon.SpanID, error) {
	bytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return pcommon.SpanID{}, err
	}
	var id pcommon.SpanID
	copy(id[:], bytes)
	return id, nil
}
