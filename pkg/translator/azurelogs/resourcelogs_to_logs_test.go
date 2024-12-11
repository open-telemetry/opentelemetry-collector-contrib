// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.22.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

var testBuildInfo = component.BuildInfo{
	Version: "1.2.3",
}

var minimumLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	lr.Attributes().PutStr(conventions.AttributeCloudResourceID, "/RESOURCE_ID")
	lr.Attributes().PutStr(eventName, eventNameValue)

	body := lr.Body().SetEmptyMap()
	body.PutStr(azureOperationName, "SecretGet")
	body.PutStr(azureCategory, "AuditEvent")
	body.CopyTo(lr.Body().Map())

	return lr
}()

var maximumLogRecord1 = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("Warning")
	guid := "607964b6-41a5-4e24-a5db-db7aab3b9b34"

	lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	lr.Attributes().PutStr(conventions.AttributeCloudResourceID, "/RESOURCE_ID-1")
	lr.Attributes().PutStr(eventName, eventNameValue)

	body := lr.Body().SetEmptyMap()
	body.PutStr(azureTenantID, "/TENANT_ID")
	body.PutStr(azureOperationName, "SecretGet")
	body.PutStr(azureOperationVersion, "7.0")
	body.PutStr(azureCategory, "AuditEvent")
	body.PutStr(azureCorrelationID, guid)
	body.PutStr(azureResultType, "Success")
	body.PutStr(azureResultSignature, "Signature")
	body.PutStr(azureResultDescription, "Description")
	body.PutInt(azureDuration, 1234)
	body.PutStr(networkPeerAddress, "127.0.0.1")
	body.PutStr(conventions.AttributeCloudRegion, "ukso")
	body.PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", guid)

	properties := body.PutEmptyMap(azureProperties)
	properties.PutStr("string", "string")
	properties.PutDouble("int", 429)
	properties.PutDouble("float", 3.14)
	properties.PutBool("bool", false)

	return lr
}()

var maximumLogRecord2 = func() []plog.LogRecord {
	sl := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr2 := sl.LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:29.6767145Z")
	lr.SetTimestamp(ts)
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("Warning")
	guid := "96317703-2132-4a8d-a5d7-e18d2f486783"

	lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	lr.Attributes().PutStr(conventions.AttributeCloudResourceID, "/RESOURCE_ID-2")
	lr.Attributes().PutStr(eventName, eventNameValue)

	body := lr.Body().SetEmptyMap()
	body.PutStr(azureTenantID, "/TENANT_ID")
	body.PutStr(azureOperationName, "SecretSet")
	body.PutStr(azureOperationVersion, "7.0")
	body.PutStr(azureCategory, "AuditEvent")
	body.PutStr(azureCorrelationID, guid)
	body.PutStr(azureResultType, "Success")
	body.PutStr(azureResultSignature, "Signature")
	body.PutStr(azureResultDescription, "Description")
	body.PutInt(azureDuration, 4321)
	body.PutStr(networkPeerAddress, "127.0.0.1")
	body.PutStr(conventions.AttributeCloudRegion, "ukso")

	body.PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", guid)
	properties := body.PutEmptyMap(azureProperties)
	properties.PutStr("string", "string")
	properties.PutDouble("int", 924)
	properties.PutDouble("float", 41.3)
	properties.PutBool("bool", true)

	ts, _ = asTimestamp("2022-11-11T04:48:31.6767145Z")
	lr2.SetTimestamp(ts)
	lr2.SetSeverityNumber(plog.SeverityNumberWarn)
	lr2.SetSeverityText("Warning")
	guid = "4ae807da-39d9-4327-b5b4-0ab685a57f9a"

	lr2.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	lr2.Attributes().PutStr(conventions.AttributeCloudResourceID, "/RESOURCE_ID-2")
	lr2.Attributes().PutStr(eventName, eventNameValue)

	body2 := lr2.Body().SetEmptyMap()
	body2.PutStr(azureTenantID, "/TENANT_ID")
	body2.PutStr(azureOperationName, "SecretGet")
	body2.PutStr(azureOperationVersion, "7.0")
	body2.PutStr(azureCategory, "AuditEvent")
	body2.PutStr(azureCorrelationID, guid)
	body2.PutStr(azureResultType, "Success")
	body2.PutStr(azureResultSignature, "Signature")
	body2.PutStr(azureResultDescription, "Description")
	body2.PutInt(azureDuration, 321)
	body2.PutStr(networkPeerAddress, "127.0.0.1")
	body2.PutStr(conventions.AttributeCloudRegion, "ukso")

	body2.PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", guid)
	properties = body2.PutEmptyMap(azureProperties)
	properties.PutStr("string", "string")
	properties.PutDouble("int", 925)
	properties.PutDouble("float", 41.4)
	properties.PutBool("bool", false)

	var records []plog.LogRecord
	return append(records, lr, lr2)
}()

var badLevelLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2023-10-26T14:22:43.3416357Z")
	lr.SetTimestamp(ts)
	lr.SetSeverityNumber(plog.SeverityNumberTrace4)
	lr.SetSeverityText("4")
	guid := "128bc026-5ead-40c7-8853-ebb32bc077a3"

	lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	lr.Attributes().PutStr(conventions.AttributeCloudResourceID, "/RESOURCE_ID")
	lr.Attributes().PutStr(eventName, eventNameValue)

	body := lr.Body().SetEmptyMap()
	body.PutStr(azureOperationName, "Microsoft.ApiManagement/GatewayLogs")
	body.PutStr(azureCategory, "GatewayLogs")
	body.PutStr(azureCorrelationID, guid)
	body.PutStr(azureResultType, "Succeeded")
	body.PutInt(azureDuration, 243)
	body.PutStr(networkPeerAddress, "13.14.15.16")
	body.PutStr(conventions.AttributeCloudRegion, "West US")

	properties := body.PutEmptyMap(azureProperties)
	properties.PutStr("method", http.MethodGet)
	properties.PutStr("url", "https://api.azure-api.net/sessions")
	properties.PutDouble("backendResponseCode", 200)
	properties.PutDouble("responseCode", 200)
	properties.PutDouble("responseSize", 102945)
	properties.PutStr("cache", "none")
	properties.PutDouble("backendTime", 54)
	properties.PutDouble("requestSize", 632)
	properties.PutStr("apiId", "demo-api")
	properties.PutStr("operationId", "GetSessions")
	properties.PutStr("apimSubscriptionId", "master")
	properties.PutDouble("clientTime", 190)
	properties.PutStr("clientProtocol", "HTTP/1.1")
	properties.PutStr("backendProtocol", "HTTP/1.1")
	properties.PutStr("apiRevision", "1")
	properties.PutStr("clientTlsVersion", "1.2")
	properties.PutStr("backendMethod", http.MethodGet)
	properties.PutStr("backendUrl", "https://api.azurewebsites.net/sessions")
	return lr
}()

var badTimeLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2021-10-14T22:17:11+00:00")
	lr.SetTimestamp(ts)

	lr.Attributes().PutStr(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAzure)
	lr.Attributes().PutStr(conventions.AttributeCloudResourceID, "/RESOURCE_ID")
	lr.Attributes().PutStr(eventName, eventNameValue)

	body := lr.Body().SetEmptyMap()
	body.PutStr(azureOperationName, "ApplicationGatewayAccess")
	body.PutStr(azureCategory, "ApplicationGatewayAccessLog")

	properties := body.PutEmptyMap(azureProperties)
	properties.PutStr("instanceId", "appgw_2")
	properties.PutStr("clientIP", "185.42.129.24")
	properties.PutDouble("clientPort", 45057)
	properties.PutStr("httpMethod", http.MethodGet)
	properties.PutStr("originalRequestUriWithArgs", "/")
	properties.PutStr("requestUri", "/")
	properties.PutStr("requestQuery", "")
	properties.PutStr("userAgent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/52.0.2743.116 Safari/537.36")
	properties.PutDouble("httpStatus", 200)
	properties.PutStr("httpVersion", "HTTP/1.1")
	properties.PutDouble("receivedBytes", 184)
	properties.PutDouble("sentBytes", 466)
	properties.PutDouble("clientResponseTime", 0)
	properties.PutDouble("timeTaken", 0.034)
	properties.PutStr("WAFEvaluationTime", "0.000")
	properties.PutStr("WAFMode", "Detection")
	properties.PutStr("transactionId", "592d1649f75a8d480a3c4dc6a975309d")
	properties.PutStr("sslEnabled", "on")
	properties.PutStr("sslCipher", "ECDHE-RSA-AES256-GCM-SHA384")
	properties.PutStr("sslProtocol", "TLSv1.2")
	properties.PutStr("sslClientVerify", "NONE")
	properties.PutStr("sslClientCertificateFingerprint", "")
	properties.PutStr("sslClientCertificateIssuerName", "")
	properties.PutStr("serverRouted", "52.239.221.65:443")
	properties.PutStr("serverStatus", "200")
	properties.PutStr("serverResponseLatency", "0.028")
	properties.PutStr("upstreamSourcePort", "21564")
	properties.PutStr("originalHost", "20.110.30.194")
	properties.PutStr("host", "20.110.30.194")
	return lr
}()

func TestAsTimestamp(t *testing.T) {
	timestamp := "2022-11-11T04:48:27.6767145Z"
	nanos, err := asTimestamp(timestamp)
	assert.NoError(t, err)
	assert.Less(t, pcommon.Timestamp(0), nanos)

	timestamp = "11/20/2024 13:57:18"
	nanos, err = asTimestamp(timestamp, "01/02/2006 15:04:05")
	assert.NoError(t, err)
	assert.Less(t, pcommon.Timestamp(0), nanos)

	// time_format set, but fallback to iso8601 and succeeded to parse
	timestamp = "2022-11-11T04:48:27.6767145Z"
	nanos, err = asTimestamp(timestamp, "01/02/2006 15:04:05")
	assert.NoError(t, err)
	assert.Less(t, pcommon.Timestamp(0), nanos)

	// time_format set, but all failed to parse
	timestamp = "11/20/2024 13:57:18"
	nanos, err = asTimestamp(timestamp, "2006-01-02 15:04:05")
	assert.Error(t, err)
	assert.Equal(t, pcommon.Timestamp(0), nanos)

	timestamp = "invalid-time"
	nanos, err = asTimestamp(timestamp)
	assert.Error(t, err)
	assert.Equal(t, pcommon.Timestamp(0), nanos)
}

func TestAsSeverity(t *testing.T) {
	tests := map[string]plog.SeverityNumber{
		"Informational": plog.SeverityNumberInfo,
		"Warning":       plog.SeverityNumberWarn,
		"Error":         plog.SeverityNumberError,
		"Critical":      plog.SeverityNumberFatal,
		"unknown":       plog.SeverityNumberUnspecified,
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, asSeverity(json.Number(input)))
		})
	}
}

func TestSetIf(t *testing.T) {
	m := map[string]any{}

	setIf(m, "key", nil)
	actual, found := m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v := ""
	setIf(m, "key", &v)
	actual, found = m["key"]
	assert.False(t, found)
	assert.Nil(t, actual)

	v = "ok"
	setIf(m, "key", &v)
	actual, found = m["key"]
	assert.True(t, found)
	assert.Equal(t, "ok", actual)
}

func TestExtractRawAttributes(t *testing.T) {
	badDuration := json.Number("invalid")
	goodDuration := json.Number("1234")

	tenantID := "tenant.id"
	operationVersion := "operation.version"
	resultType := "result.type"
	resultSignature := "result.signature"
	resultDescription := "result.description"
	callerIPAddress := "127.0.0.1"
	correlationID := "edb70d1a-eec2-4b4c-b2f4-60e3510160ee"
	level := json.Number("Informational")
	location := "location"

	identity := any("someone")

	properties := any(map[string]any{
		"a": uint64(1),
		"b": true,
		"c": 1.23,
		"d": "ok",
	})

	stringProperties := any("str")
	intProperties := any(1)
	jsonProperties := any("{\"a\": 1, \"b\": true, \"c\": 1.23, \"d\": \"ok\"}")

	tests := []struct {
		name     string
		log      azureLogRecord
		expected map[string]any
	}{
		{
			name: "minimal",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
			},
		},
		{
			name: "bad-duration",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
			},
		},
		{
			name: "everything",
			log: azureLogRecord{
				Time:              "",
				ResourceID:        "resource.id",
				TenantID:          &tenantID,
				OperationName:     "operation.name",
				OperationVersion:  &operationVersion,
				Category:          "category",
				ResultType:        &resultType,
				ResultSignature:   &resultSignature,
				ResultDescription: &resultDescription,
				DurationMs:        &goodDuration,
				CallerIPAddress:   &callerIPAddress,
				CorrelationID:     &correlationID,
				Identity:          &identity,
				Level:             &level,
				Location:          &location,
				Properties:        &properties,
			},
			expected: map[string]any{
				azureTenantID:                    "tenant.id",
				azureOperationName:               "operation.name",
				azureOperationVersion:            "operation.version",
				azureCategory:                    "category",
				azureCorrelationID:               correlationID,
				azureResultType:                  "result.type",
				azureResultSignature:             "result.signature",
				azureResultDescription:           "result.description",
				azureDuration:                    int64(1234),
				networkPeerAddress:               "127.0.0.1",
				azureIdentity:                    "someone",
				conventions.AttributeCloudRegion: "location",
				azureProperties:                  properties,
			},
		},
		{
			name: "nil properties",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    nil,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
			},
		},
		{
			name: "string properties",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    &stringProperties,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				azureProperties:    "str",
			},
		},
		{
			name: "int properties",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    &intProperties,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				azureProperties:    1,
			},
		},
		{
			name: "json properties",
			log: azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    &jsonProperties,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				azureProperties:    "{\"a\": 1, \"b\": true, \"c\": 1.23, \"d\": \"ok\"}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractRawAttributes(tt.log))
		})
	}
}

func TestUnmarshalLogs(t *testing.T) {
	expectedMinimum := plog.NewLogs()
	resourceLogs := expectedMinimum.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr := scopeLogs.LogRecords().AppendEmpty()
	minimumLogRecord.CopyTo(lr)

	expectedMinimum2 := plog.NewLogs()
	resourceLogs = expectedMinimum2.ResourceLogs().AppendEmpty()
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	logRecords := scopeLogs.LogRecords()
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)

	expectedMaximum := plog.NewLogs()
	resourceLogs = expectedMaximum.ResourceLogs().AppendEmpty()
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	maximumLogRecord1.CopyTo(lr)

	resourceLogs = expectedMaximum.ResourceLogs().AppendEmpty()
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	lr2 := scopeLogs.LogRecords().AppendEmpty()
	maximumLogRecord2[0].CopyTo(lr)
	maximumLogRecord2[1].CopyTo(lr2)

	expectedBadLevel := plog.NewLogs()
	resourceLogs = expectedBadLevel.ResourceLogs().AppendEmpty()
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	badLevelLogRecord.CopyTo(lr)

	expectedBadTime := plog.NewLogs()
	resourceLogs = expectedBadTime.ResourceLogs().AppendEmpty()
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("otelcol/azureresourcelogs")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	badTimeLogRecord.CopyTo(lr)

	tests := []struct {
		file     string
		expected plog.Logs
	}{
		{
			file:     "log-minimum.json",
			expected: expectedMinimum,
		},
		{
			file:     "log-minimum-2.json",
			expected: expectedMinimum2,
		},
		{
			file:     "log-maximum.json",
			expected: expectedMaximum,
		},
		{
			file:     "log-bad-level.json",
			expected: expectedBadLevel,
		},
		{
			file:     "log-bad-time.json",
			expected: expectedBadTime,
		},
	}

	sut := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			assert.NoError(t, err)
			assert.NotNil(t, data)

			logs, err := sut.UnmarshalLogs(data)
			assert.NoError(t, err)

			assert.NoError(t, plogtest.CompareLogs(tt.expected, logs))
		})
	}
}

func loadJSONLogsAndApplySemanticConventions(filename string) (plog.Logs, error) {
	l := plog.NewLogs()

	sut := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}

	data, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		return l, err
	}

	logs, err := sut.UnmarshalLogs(data)
	if err != nil {
		return l, err
	}

	return logs, nil
}

func TestAzureCdnAccessLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-azurecdnaccesslog.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, http.MethodGet, record["http.request.method"])
	assert.Equal(t, "1.1.0.0", record["network.protocol.version"])
	assert.Equal(t, "TRACKING_REFERENCE", record["az.service_request_id"])
	assert.Equal(t, "https://test.net/", record["url.full"])
	assert.Equal(t, int64(1234), record["http.request.size"])
	assert.Equal(t, int64(12345), record["http.response.size"])
	assert.Equal(t, "Mozilla/5.0", record["user_agent.original"])
	assert.Equal(t, "42.42.42.42", record["client.address"])
	assert.Equal(t, "0", record["client.port"])
	assert.Equal(t, "tls", record["tls.protocol.name"])
	assert.Equal(t, "1.3", record["tls.protocol.version"])
	assert.Equal(t, int64(200), record["http.response.status_code"])
	assert.Equal(t, "NoError", record["error.type"])
}

func TestFrontDoorAccessLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-frontdooraccesslog.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, http.MethodGet, record["http.request.method"])
	assert.Equal(t, "1.1.0.0", record["network.protocol.version"])
	assert.Equal(t, "TRACKING_REFERENCE", record["az.service_request_id"])
	assert.Equal(t, "https://test.net/", record["url.full"])
	assert.Equal(t, int64(1234), record["http.request.size"])
	assert.Equal(t, int64(12345), record["http.response.size"])
	assert.Equal(t, "Mozilla/5.0", record["user_agent.original"])
	assert.Equal(t, "42.42.42.42", record["client.address"])
	assert.Equal(t, "0", record["client.port"])
	assert.Equal(t, "23.23.23.23", record["network.peer.address"])
	assert.Equal(t, float64(0.23), record["http.server.request.duration"])
	assert.Equal(t, "https", record["network.protocol.name"])
	assert.Equal(t, "tls", record["tls.protocol.name"])
	assert.Equal(t, "1.3", record["tls.protocol.version"])
	assert.Equal(t, "TLS_AES_256_GCM_SHA384", record["tls.cipher"])
	assert.Equal(t, "secp384r1", record["tls.curve"])
	assert.Equal(t, int64(200), record["http.response.status_code"])
	assert.Equal(t, "REFERER", record["http.request.header.referer"])
	assert.Equal(t, "NoError", record["error.type"])
}

func TestFrontDoorHealthProbeLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-frontdoorhealthprobelog.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, http.MethodGet, record["http.request.method"])
	assert.Equal(t, int64(200), record["http.response.status_code"])
	assert.Equal(t, "https://probe.net/health", record["url.full"])
	assert.Equal(t, "42.42.42.42", record["server.address"])
	assert.Equal(t, 0.042, record["http.request.duration"])
	assert.Equal(t, 0.00023, record["dns.lookup.duration"])
}

func TestFrontDoorWAFLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-frontdoorwaflog.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, "TRACKING_REFERENCE", record["az.service_request_id"])
	assert.Equal(t, "https://test.net/", record["url.full"])
	assert.Equal(t, "test.net", record["server.address"])
	assert.Equal(t, "42.42.42.42", record["client.address"])
	assert.Equal(t, "0", record["client.port"])
	assert.Equal(t, "23.23.23.23", record["network.peer.address"])
}

func TestAppServiceAppLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-appserviceapplogs.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, "CONTAINER_ID", record["container.id"])
	assert.Equal(t, "EXCEPTION_CLASS", record["exception.type"])
	assert.Equal(t, "HOST", record["host.id"])
	assert.Equal(t, "METHOD", record["code.function"])
	assert.Equal(t, "FILEPATH", record["code.filepath"])
	assert.Equal(t, "STACKTRACE", record["exception.stacktrace"])
}

func TestAppServiceConsoleLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-appserviceconsolelogs.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, "CONTAINER_ID", record["container.id"])
	assert.Equal(t, "HOST", record["host.id"])
}

func TestAppServiceAuditLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-appserviceauditlogs.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, "USER_ID", record["enduser.id"])
	assert.Equal(t, "42.42.42.42", record["client.address"])
	assert.Equal(t, "kudu", record["network.protocol.name"])
}

func TestAppServiceHTTPLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-appservicehttplogs.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, "test.com", record["url.domain"])
	assert.Equal(t, "42.42.42.42", record["client.address"])
	assert.Equal(t, int64(80), record["server.port"])
	assert.Equal(t, "/api/test/", record["url.path"])
	assert.Equal(t, "foo=42", record["url.query"])
	assert.Equal(t, http.MethodGet, record["http.request.method"])
	assert.Equal(t, 0.42, record["http.server.request.duration"])
	assert.Equal(t, int64(200), record["http.response.status_code"])
	assert.Equal(t, int64(4242), record["http.request.body.size"])
	assert.Equal(t, int64(42), record["http.response.body.size"])
	assert.Equal(t, "Mozilla/5.0", record["user_agent.original"])
	assert.Equal(t, "REFERER", record["http.request.header.referer"])
	assert.Equal(t, "COMPUTER_NAME", record["host.name"])
	assert.Equal(t, "http", record["network.protocol.name"])
	assert.Equal(t, "1.1", record["network.protocol.version"])
}

func TestAppServicePlatformLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-appserviceplatformlogs.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, "CONTAINER_ID", record["container.id"])
	assert.Equal(t, "CONTAINER_NAME", record["container.name"])
}

func TestAppServiceIPSecAuditLog(t *testing.T) {
	logs, err := loadJSONLogsAndApplySemanticConventions("log-appserviceipsecauditlogs.json")

	assert.NoError(t, err)

	record := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Map().AsRaw()

	assert.Equal(t, "42.42.42.42", record["client.address"])
	assert.Equal(t, "HOST", record["url.domain"])
	assert.Equal(t, "FDID", record["http.request.header.x-azure-fdid"])
	assert.Equal(t, "HEALTH_PROBE", record["http.request.header.x-fd-healthprobe"])
	assert.Equal(t, "FORWARDED_FOR", record["http.request.header.x-forwarded-for"])
	assert.Equal(t, "FORWARDED_HOST", record["http.request.header.x-forwarded-host"])
}
