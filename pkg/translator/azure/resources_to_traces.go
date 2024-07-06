package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver"

import (
	"bytes"

	"encoding/hex"
	"net/url"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.uber.org/zap"
)

const (
	// Constants for OpenTelemetry Specs
	traceAzureResourceID = "azure.resource.id"
	traceScopeName       = "otelcol/azureresourcetraces"
)

type azureTracesRecords struct {
	Records []azureTracesRecord `json:"records"`
}

// Azure Trace Records based on Azure AppRequests & AppDependencies table data
// the common record schema reference:
// https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/apprequests
// https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appdependencies
type azureTracesRecord struct {
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
	OperationId           string             `json:"OperationId"`
	ParentId              string             `json:"ParentId"`
	SDKVersion            string             `json:"SDKVersion"`
	Properties            map[string]string  `json:"Properties"`
	Measurements          map[string]float64 `json:"Measurements"`
	SpanId                string             `json:"Id"`
	Name                  string             `json:"Name"`
	Url                   string             `json:"Url"`
	Source                string             `json:"Source"`
	Success               bool               `json:"Success"`
	ResultCode            string             `json:"ResultCode"`
	DurationMs            float64            `json:"DurationMs"`
	PerformanceBucket     string             `json:"PerformanceBucket"`
	ItemCount             float64            `json:"ItemCount"`
}

var _ ptrace.Unmarshaler = (*TracesUnmarshaler)(nil)

type TracesUnmarshaler struct {
	Version string
	Logger  *zap.Logger
}

func (r TracesUnmarshaler) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	t := ptrace.NewTraces()

	var azureTraces azureTracesRecords
	decoder := jsoniter.NewDecoder(bytes.NewReader(buf))
	err := decoder.Decode(&azureTraces)
	if err != nil {
		return t, err
	}

	resourceTraces := t.ResourceSpans().AppendEmpty()
	resource := resourceTraces.Resource()
	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKName, traceScopeName)
	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKLanguage, conventions.AttributeTelemetrySDKLanguageGo)
	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKVersion, r.Version)
	resource.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)

	scopeSpans := resourceTraces.ScopeSpans().AppendEmpty()

	spans := scopeSpans.Spans()

	resourceID := ""
	for _, azureTrace := range azureTraces.Records {
		if resourceID == "" && azureTrace.ResourceID != "" {
			resourceID = azureTrace.ResourceID
		}

		resource.Attributes().PutStr("service.name", azureTrace.AppRoleName)

		nanos, err := asTimestamp(azureTrace.Time)
		if err != nil {
			r.Logger.Warn("Invalid Timestamp", zap.String("time", azureTrace.Time))
			continue
		}

		var trace_id, trace_err = TraceIDFromHex(azureTrace.OperationId)
		if trace_err != nil {
			r.Logger.Warn("Invalid TraceID", zap.String("trace_id", azureTrace.OperationId))
			return t, err
		}
		var span_id, span_err = SpanIDFromHex(azureTrace.SpanId)
		if span_err != nil {
			r.Logger.Warn("Invalid SpanID", zap.String("span_id", azureTrace.SpanId))
			return t, err
		}
		var parent_id, parent_err = SpanIDFromHex(azureTrace.ParentId)
		if parent_err != nil {
			r.Logger.Warn("Invalid ParentID", zap.String("parent_id", azureTrace.ParentId))
			return t, err
		}

		span := spans.AppendEmpty()
		span.SetTraceID(trace_id)
		span.SetSpanID(span_id)
		span.SetParentSpanID(parent_id)

		span.Attributes().PutStr("OperationName", azureTrace.OperationName)
		span.Attributes().PutStr("AppRoleName", azureTrace.AppRoleName)
		span.Attributes().PutStr("AppRoleInstance", azureTrace.AppRoleInstance)
		span.Attributes().PutStr("Type", azureTrace.Type)

		span.Attributes().PutStr("http.url", azureTrace.Url)

		urlObj, _ := url.Parse(azureTrace.Url)
		hostname := urlObj.Host
		hostpath := urlObj.Path
		scheme := urlObj.Scheme

		span.Attributes().PutStr("http.host", hostname)
		span.Attributes().PutStr("http.path", hostpath)
		span.Attributes().PutStr("http.response.status_code", azureTrace.ResultCode)
		span.Attributes().PutStr("http.client_ip", azureTrace.ClientIP)
		span.Attributes().PutStr("http.client_city", azureTrace.ClientCity)
		span.Attributes().PutStr("http.client_type", azureTrace.ClientType)
		span.Attributes().PutStr("http.client_state", azureTrace.ClientStateOrProvince)
		span.Attributes().PutStr("http.client_type", azureTrace.ClientType)
		span.Attributes().PutStr("http.client_country", azureTrace.ClientCountryOrRegion)
		span.Attributes().PutStr("http.scheme", scheme)
		span.Attributes().PutStr("http.method", azureTrace.Properties["HTTP Method"])

		span.SetKind(ptrace.SpanKindServer)
		span.SetName(azureTrace.Name)
		span.SetStartTimestamp(nanos)
		span.SetEndTimestamp(nanos + pcommon.Timestamp(azureTrace.DurationMs*1e6))
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
