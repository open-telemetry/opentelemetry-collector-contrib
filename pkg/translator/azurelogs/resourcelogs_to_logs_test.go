// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

var testBuildInfo = component.BuildInfo{
	Version: "1.2.3",
}

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
				azureTenantID:                           "tenant.id",
				azureOperationName:                      "operation.name",
				azureOperationVersion:                   "operation.version",
				azureCategory:                           "category",
				azureCorrelationID:                      correlationID,
				azureResultType:                         "result.type",
				azureResultSignature:                    "result.signature",
				azureResultDescription:                  "result.description",
				azureDuration:                           int64(1234),
				conventions.AttributeNetworkPeerAddress: "127.0.0.1",
				azureIdentity:                           "someone",
				conventions.AttributeCloudRegion:        "location",
				azureProperties:                         properties,
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

func TestUnmarshalLogs_Files(t *testing.T) {
	t.Parallel()

	logsDir := "testdata"
	expectedDir := "testdata/expected"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
	}{
		"app_logs": {
			logFilename:      "log-appserviceapplogs.json",
			expectedFilename: "service-app-expected.yaml",
		},
		"audit_logs": {
			logFilename:      "log-appserviceauditlogs.json",
			expectedFilename: "audit-logs-expected.yaml",
		},
		"audit_logs_2": {
			logFilename:      "log-appserviceipsecauditlogs.json",
			expectedFilename: "audit-logs-2-expected.yaml",
		},
		"console_logs": {
			logFilename:      "log-appserviceconsolelogs.json",
			expectedFilename: "console-logs-expected.yaml",
		},
		"http_logs": {
			logFilename:      "log-appservicehttplogs.json",
			expectedFilename: "http-logs-expected.yaml",
		},
		"platform_logs": {
			logFilename:      "log-appserviceplatformlogs.json",
			expectedFilename: "platform-logs-expected.yaml",
		},
		"access_logs": {
			logFilename:      "log-azurecdnaccesslog.json",
			expectedFilename: "access-log-expected.yaml",
		},
		"front_door_access_logs": {
			logFilename:      "log-frontdooraccesslog.json",
			expectedFilename: "front-door-access-log-expected.yaml",
		},
		"front_door_health_probe_logs": {
			logFilename:      "log-frontdoorhealthprobelog.json",
			expectedFilename: "front-door-health-probe-log-expected.yaml",
		},
		"front_door_way_logs": {
			logFilename:      "log-frontdoorwaflog.json",
			expectedFilename: "front-door-waf-log-expected.yaml",
		},
		"log_bad_time": {
			logFilename:      "log-bad-time.json",
			expectedFilename: "log-bad-time-expected.yaml",
		},
		"log_bad_level": {
			logFilename:      "log-bad-level.json",
			expectedFilename: "log-bad-level-expected.yaml",
		},
		"log_maximum": {
			logFilename:      "log-maximum.json",
			expectedFilename: "log-maximum-expected.yaml",
		},
		"log_minimum": {
			logFilename:      "log-minimum.json",
			expectedFilename: "log-minimum-expected.yaml",
		},
		"log_minimum_2": {
			logFilename:      "log-minimum-2.json",
			expectedFilename: "log-minimum-2-expected.yaml",
		},
	}

	u := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(logsDir, test.logFilename))
			require.NoError(t, err)

			logs, err := u.UnmarshalLogs(data)
			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(expectedDir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder()))
		})
	}
}
