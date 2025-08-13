// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azure // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure"

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
	conventions "go.opentelemetry.io/otel/semconv/v1.13.0"
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
	lr.Attributes().PutStr(azureOperationName, "SecretGet")
	lr.Attributes().PutStr(azureCategory, "AuditEvent")
	lr.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())
	return lr
}()

var maximumLogRecord1 = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2022-11-11T04:48:27.6767145Z")
	lr.SetTimestamp(ts)
	lr.SetSeverityNumber(plog.SeverityNumberWarn)
	lr.SetSeverityText("Warning")
	guid := "607964b6-41a5-4e24-a5db-db7aab3b9b34"

	lr.Attributes().PutStr(azureTenantID, "/TENANT_ID")
	lr.Attributes().PutStr(azureOperationName, "SecretGet")
	lr.Attributes().PutStr(azureOperationVersion, "7.0")
	lr.Attributes().PutStr(azureCategory, "AuditEvent")
	lr.Attributes().PutStr(azureCorrelationID, guid)
	lr.Attributes().PutStr(azureResultType, "Success")
	lr.Attributes().PutStr(azureResultSignature, "Signature")
	lr.Attributes().PutStr(azureResultDescription, "Description")
	lr.Attributes().PutInt(azureDuration, 1234)
	lr.Attributes().PutStr(string(conventions.NetSockPeerAddrKey), "127.0.0.1")
	lr.Attributes().PutStr(string(conventions.CloudRegionKey), "ukso")
	lr.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	lr.Attributes().PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", guid)
	m := lr.Attributes().PutEmptyMap(azureProperties)
	m.PutStr("string", "string")
	m.PutDouble("int", 429)
	m.PutDouble("float", 3.14)
	m.PutBool("bool", false)

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

	lr.Attributes().PutStr(azureTenantID, "/TENANT_ID")
	lr.Attributes().PutStr(azureOperationName, "SecretSet")
	lr.Attributes().PutStr(azureOperationVersion, "7.0")
	lr.Attributes().PutStr(azureCategory, "AuditEvent")
	lr.Attributes().PutStr(azureCorrelationID, guid)
	lr.Attributes().PutStr(azureResultType, "Success")
	lr.Attributes().PutStr(azureResultSignature, "Signature")
	lr.Attributes().PutStr(azureResultDescription, "Description")
	lr.Attributes().PutInt(azureDuration, 4321)
	lr.Attributes().PutStr(string(conventions.NetSockPeerAddrKey), "127.0.0.1")
	lr.Attributes().PutStr(string(conventions.CloudRegionKey), "ukso")
	lr.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	lr.Attributes().PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", guid)
	m := lr.Attributes().PutEmptyMap(azureProperties)
	m.PutStr("string", "string")
	m.PutDouble("int", 924)
	m.PutDouble("float", 41.3)
	m.PutBool("bool", true)

	ts, _ = asTimestamp("2022-11-11T04:48:31.6767145Z")
	lr2.SetTimestamp(ts)
	lr2.SetSeverityNumber(plog.SeverityNumberWarn)
	lr2.SetSeverityText("Warning")
	guid = "4ae807da-39d9-4327-b5b4-0ab685a57f9a"

	lr2.Attributes().PutStr(azureTenantID, "/TENANT_ID")
	lr2.Attributes().PutStr(azureOperationName, "SecretGet")
	lr2.Attributes().PutStr(azureOperationVersion, "7.0")
	lr2.Attributes().PutStr(azureCategory, "AuditEvent")
	lr2.Attributes().PutStr(azureCorrelationID, guid)
	lr2.Attributes().PutStr(azureResultType, "Success")
	lr2.Attributes().PutStr(azureResultSignature, "Signature")
	lr2.Attributes().PutStr(azureResultDescription, "Description")
	lr2.Attributes().PutInt(azureDuration, 321)
	lr2.Attributes().PutStr(string(conventions.NetSockPeerAddrKey), "127.0.0.1")
	lr2.Attributes().PutStr(string(conventions.CloudRegionKey), "ukso")
	lr2.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	lr2.Attributes().PutEmptyMap(azureIdentity).PutEmptyMap("claim").PutStr("oid", guid)
	m = lr2.Attributes().PutEmptyMap(azureProperties)
	m.PutStr("string", "string")
	m.PutDouble("int", 925)
	m.PutDouble("float", 41.4)
	m.PutBool("bool", false)

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

	lr.Attributes().PutStr(azureOperationName, "Microsoft.ApiManagement/GatewayLogs")
	lr.Attributes().PutStr(azureCategory, "GatewayLogs")
	lr.Attributes().PutStr(azureCorrelationID, guid)
	lr.Attributes().PutStr(azureResultType, "Succeeded")
	lr.Attributes().PutInt(azureDuration, 243)
	lr.Attributes().PutStr(string(conventions.NetSockPeerAddrKey), "13.14.15.16")
	lr.Attributes().PutStr(string(conventions.CloudRegionKey), "West US")
	lr.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	m := lr.Attributes().PutEmptyMap(azureProperties)
	m.PutStr("method", http.MethodGet)
	m.PutStr("url", "https://api.azure-api.net/sessions")
	m.PutDouble("backendResponseCode", 200)
	m.PutDouble("responseCode", 200)
	m.PutDouble("responseSize", 102945)
	m.PutStr("cache", "none")
	m.PutDouble("backendTime", 54)
	m.PutDouble("requestSize", 632)
	m.PutStr("apiId", "demo-api")
	m.PutStr("operationId", "GetSessions")
	m.PutStr("apimSubscriptionId", "master")
	m.PutDouble("clientTime", 190)
	m.PutStr("clientProtocol", "HTTP/1.1")
	m.PutStr("backendProtocol", "HTTP/1.1")
	m.PutStr("apiRevision", "1")
	m.PutStr("clientTlsVersion", "1.2")
	m.PutStr("backendMethod", http.MethodGet)
	m.PutStr("backendUrl", "https://api.azurewebsites.net/sessions")
	return lr
}()

var badTimeLogRecord = func() plog.LogRecord {
	lr := plog.NewLogs().ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	ts, _ := asTimestamp("2021-10-14T22:17:11+00:00")
	lr.SetTimestamp(ts)

	lr.Attributes().PutStr(azureOperationName, "ApplicationGatewayAccess")
	lr.Attributes().PutStr(azureCategory, "ApplicationGatewayAccessLog")
	lr.Attributes().PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAzure.Value.AsString())

	m := lr.Attributes().PutEmptyMap(azureProperties)
	m.PutStr("instanceId", "appgw_2")
	m.PutStr("clientIP", "185.42.129.24")
	m.PutDouble("clientPort", 45057)
	m.PutStr("httpMethod", http.MethodGet)
	m.PutStr("originalRequestUriWithArgs", "/")
	m.PutStr("requestUri", "/")
	m.PutStr("requestQuery", "")
	m.PutStr("userAgent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/52.0.2743.116 Safari/537.36")
	m.PutDouble("httpStatus", 200)
	m.PutStr("httpVersion", "HTTP/1.1")
	m.PutDouble("receivedBytes", 184)
	m.PutDouble("sentBytes", 466)
	m.PutDouble("clientResponseTime", 0)
	m.PutDouble("timeTaken", 0.034)
	m.PutStr("WAFEvaluationTime", "0.000")
	m.PutStr("WAFMode", "Detection")
	m.PutStr("transactionId", "592d1649f75a8d480a3c4dc6a975309d")
	m.PutStr("sslEnabled", "on")
	m.PutStr("sslCipher", "ECDHE-RSA-AES256-GCM-SHA384")
	m.PutStr("sslProtocol", "TLSv1.2")
	m.PutStr("sslClientVerify", "NONE")
	m.PutStr("sslClientCertificateFingerprint", "")
	m.PutStr("sslClientCertificateIssuerName", "")
	m.PutStr("serverRouted", "52.239.221.65:443")
	m.PutStr("serverStatus", "200")
	m.PutStr("serverResponseLatency", "0.028")
	m.PutStr("upstreamSourcePort", "21564")
	m.PutStr("originalHost", "20.110.30.194")
	m.PutStr("host", "20.110.30.194")
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
	nanos, err = asTimestamp(timestamp, nil...)
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
				azureOperationName:                   "operation.name",
				azureCategory:                        "category",
				string(conventions.CloudProviderKey): conventions.CloudProviderAzure.Value.AsString(),
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
				azureOperationName:                   "operation.name",
				azureCategory:                        "category",
				string(conventions.CloudProviderKey): conventions.CloudProviderAzure.Value.AsString(),
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
				azureTenantID:                          "tenant.id",
				azureOperationName:                     "operation.name",
				azureOperationVersion:                  "operation.version",
				azureCategory:                          "category",
				azureCorrelationID:                     correlationID,
				azureResultType:                        "result.type",
				azureResultSignature:                   "result.signature",
				azureResultDescription:                 "result.description",
				azureDuration:                          int64(1234),
				string(conventions.NetSockPeerAddrKey): "127.0.0.1",
				azureIdentity:                          "someone",
				string(conventions.CloudRegionKey):     "location",
				string(conventions.CloudProviderKey):   conventions.CloudProviderAzure.Value.AsString(),
				azureProperties:                        properties,
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
	scopeLogs.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr := scopeLogs.LogRecords().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	minimumLogRecord.CopyTo(lr)

	expectedMinimum2 := plog.NewLogs()
	resourceLogs = expectedMinimum2.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	logRecords := scopeLogs.LogRecords()
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)
	lr = logRecords.AppendEmpty()
	minimumLogRecord.CopyTo(lr)

	expectedMaximum := plog.NewLogs()
	resourceLogs = expectedMaximum.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID-1")
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	maximumLogRecord1.CopyTo(lr)

	resourceLogs = expectedMaximum.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID-2")
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	lr2 := scopeLogs.LogRecords().AppendEmpty()
	maximumLogRecord2[0].CopyTo(lr)
	maximumLogRecord2[1].CopyTo(lr2)

	expectedBadLevel := plog.NewLogs()
	resourceLogs = expectedBadLevel.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure")
	scopeLogs.Scope().SetVersion(testBuildInfo.Version)
	lr = scopeLogs.LogRecords().AppendEmpty()
	badLevelLogRecord.CopyTo(lr)

	expectedBadTime := plog.NewLogs()
	resourceLogs = expectedBadTime.ResourceLogs().AppendEmpty()
	resourceLogs.Resource().Attributes().PutStr(azureResourceID, "/RESOURCE_ID")
	scopeLogs = resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName("github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure")
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
