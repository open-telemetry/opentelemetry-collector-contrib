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

	identity := json.RawMessage(`"someone"`)

	properties := map[string]any{
		"a": float64(1),
		"b": true,
		"c": 1.23,
		"d": "ok",
	}
	propertiesRaw, err := json.Marshal(properties)
	require.NoError(t, err)

	stringProperties := "str"
	stringPropertiesRaw, err := json.Marshal(stringProperties)
	require.NoError(t, err)

	intProperties := 1
	intPropertiesRaw, err := json.Marshal(intProperties)
	require.NoError(t, err)

	jsonProperties := "{\"a\": 1, \"b\": true, \"c\": 1.23, \"d\": \"ok\"}"
	jsonPropertiesRaw, err := json.Marshal(jsonProperties)
	require.NoError(t, err)

	tests := []struct {
		name     string
		log      *azureLogRecord
		expected map[string]any
	}{
		{
			name: "minimal",
			log: &azureLogRecord{
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
			log: &azureLogRecord{
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
			log: &azureLogRecord{
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
				Identity:          identity,
				Level:             &level,
				Location:          &location,
				Properties:        propertiesRaw,
			},
			expected: map[string]any{
				azureTenantID:          "tenant.id",
				azureOperationName:     "operation.name",
				azureOperationVersion:  "operation.version",
				azureCategory:          "category",
				azureCorrelationID:     correlationID,
				azureResultType:        "result.type",
				azureResultSignature:   "result.signature",
				azureResultDescription: "result.description",
				azureDuration:          int64(1234),
				"network.peer.address": "127.0.0.1",
				azureIdentity:          "someone",
				"cloud.region":         "location",
				azureProperties:        properties,
			},
		},
		{
			name: "nil properties",
			log: &azureLogRecord{
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
			log: &azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    stringPropertiesRaw,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				azureProperties:    "str",
			},
		},
		{
			name: "int properties",
			log: &azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    intPropertiesRaw,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				azureProperties:    float64(1),
			},
		},
		{
			name: "json properties",
			log: &azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    jsonPropertiesRaw,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				azureProperties:    "{\"a\": 1, \"b\": true, \"c\": 1.23, \"d\": \"ok\"}",
			},
		},
		{
			name: "unknown fields",
			log: &azureLogRecord{
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
			name: "primitive properties with unknown",
			log: &azureLogRecord{
				Time:          "",
				ResourceID:    "resource.id",
				OperationName: "operation.name",
				Category:      "category",
				DurationMs:    &badDuration,
				Properties:    stringPropertiesRaw,
			},
			expected: map[string]any{
				azureOperationName: "operation.name",
				azureCategory:      "category",
				azureProperties:    "str",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractRawAttributes(tt.log, nil))
		})
	}
}

func TestUnmarshalLogs_AzureCdnAccessLog(t *testing.T) {
	t.Parallel()

	dir := "testdata/cdnaccesslog"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
		expectsErr       string
	}{
		"valid_1": {
			logFilename:      "valid_1.json",
			expectedFilename: "valid_1_expected.yaml",
		},
		"valid_2": {
			logFilename:      "valid_2.json",
			expectedFilename: "valid_2_expected.yaml",
		},
		"valid_3": {
			logFilename:      "valid_3.json",
			expectedFilename: "valid_3_expected.yaml",
		},
	}

	u := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, test.logFilename))
			require.NoError(t, err)

			logs, err := u.UnmarshalLogs(data)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder(), plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestUnmarshalLogs_FrontDoorWebApplicationFirewallLog(t *testing.T) {
	t.Parallel()

	dir := "testdata/frontdoorwebapplicationfirewalllog"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
		expectsErr       string
	}{
		"valid_1": {
			logFilename:      "valid_1.json",
			expectedFilename: "valid_1_expected.yaml",
		},
	}

	u := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, test.logFilename))
			require.NoError(t, err)

			logs, err := u.UnmarshalLogs(data)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder(), plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestUnmarshalLogs_FrontDoorAccessLog(t *testing.T) {
	t.Parallel()

	dir := "testdata/frontdooraccesslog"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
		expectsErr       string
	}{
		"valid_1": {
			logFilename:      "valid_1.json",
			expectedFilename: "valid_1_expected.yaml",
		},
	}

	u := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, test.logFilename))
			require.NoError(t, err)

			logs, err := u.UnmarshalLogs(data)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder(), plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestUnmarshalLogs_VNetFlowLog(t *testing.T) {
	t.Parallel()

	dir := "testdata/azurevnetflowlog"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
		expectsErr       string
	}{
		"valid_1": {
			logFilename:      "valid_1.json",
			expectedFilename: "valid_1_expected.yaml",
		},
	}

	u := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, test.logFilename))
			require.NoError(t, err)

			logs, err := u.UnmarshalLogs(data)

			if test.expectsErr != "" {
				require.ErrorContains(t, err, test.expectsErr)
				return
			}

			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder(), plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestUnmarshalLogs_Files(t *testing.T) {
	// TODO @constanca-m Eventually this test function will be fully
	// replaced with TestUnmarshalLogs_<category>, once all the currently supported
	// categories are handled in category_logs.log

	t.Parallel()

	logsDir := "testdata"
	expectedDir := "testdata"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
	}{
		"app_logs": {
			logFilename:      "appservicelog/appservice_applogs.json",
			expectedFilename: "appservicelog/appservice_applogs_expected.yaml",
		},
		"audit_logs": {
			logFilename:      "appservicelog/appservice_auditlogs.json",
			expectedFilename: "appservicelog/appservice_auditlogs_expected.yaml",
		},
		"audit_logs_2": {
			logFilename:      "appservicelog/appservice_ipsecauditlogs.json",
			expectedFilename: "appservicelog/appservice_ipsecauditlogs_expected.yaml",
		},
		"console_logs": {
			logFilename:      "appservicelog/appservice_consolelogs.json",
			expectedFilename: "appservicelog/appservice_consolelogs_expected.yaml",
		},
		"http_logs": {
			logFilename:      "appservicelog/appservice_httplogs.json",
			expectedFilename: "appservicelog/appservice_httplogs_expected.yaml",
		},
		"platform_logs": {
			logFilename:      "appservicelog/appservice_platformlogs.json",
			expectedFilename: "appservicelog/appservice_platformlogs_expected.yaml",
		},
		"front_door_health_probe_logs": {
			logFilename:      "frontdoorhealthprobelog/valid_1.json",
			expectedFilename: "frontdoorhealthprobelog/valid_1_expected.yaml",
		},
		"log_bad_time": {
			logFilename:      "cornercases/bad_time.json",
			expectedFilename: "cornercases/bad_time_expected.yaml",
		},
		"log_bad_level": {
			logFilename:      "cornercases/bad_level.json",
			expectedFilename: "cornercases/bad_level_expected.yaml",
		},
		"log_maximum": {
			logFilename:      "cornercases/maximum.json",
			expectedFilename: "cornercases/maximum_expected.yaml",
		},
		"log_minimum": {
			logFilename:      "cornercases/minimum.json",
			expectedFilename: "cornercases/minimum_expected.yaml",
		},
		"log_minimum_2": {
			logFilename:      "cornercases/minimum-2.json",
			expectedFilename: "cornercases/minimum-2_expected.yaml",
		},
		"log_identity_as_string": {
			logFilename:      "cornercases/identity_as_string.json",
			expectedFilename: "cornercases/identity_as_string_expected.yaml",
		},
		"log_identity_as_object": {
			logFilename:      "cornercases/identity_as_object.json",
			expectedFilename: "cornercases/identity_as_object_expected.yaml",
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
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder(), plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestUnmarshalLogs_Recommendation(t *testing.T) {
	t.Parallel()

	dir := "testdata/recommendation"
	tests := map[string]struct {
		logFilename      string
		expectedFilename string
	}{
		"valid_1": {
			logFilename:      "valid_1.json",
			expectedFilename: "valid_1_expected.yaml",
		},
	}

	u := &ResourceLogsUnmarshaler{
		Version: testBuildInfo.Version,
		Logger:  zap.NewNop(),
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, test.logFilename))
			require.NoError(t, err)

			logs, err := u.UnmarshalLogs(data)
			require.NoError(t, err)

			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, test.expectedFilename))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreResourceLogsOrder(), plogtest.IgnoreObservedTimestamp()))
		})
	}
}
