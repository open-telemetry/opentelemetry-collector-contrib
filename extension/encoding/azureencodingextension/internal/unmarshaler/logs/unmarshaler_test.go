// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const testFilesDirectory = "testdata"

var testBuildInfo = component.BuildInfo{
	Version: "test-version",
}

func TestResourceLogsUnmarshaler_UnmarshalLogs_Golden(t *testing.T) {
	t.Parallel()

	testCategoriesDirs, err := os.ReadDir(testFilesDirectory)
	require.NoError(t, err)

	for _, dir := range testCategoriesDirs {
		if !dir.IsDir() {
			continue
		}

		category := dir.Name()
		t.Run(category, func(t *testing.T) {
			t.Parallel()

			testFiles, err := filepath.Glob(filepath.Join(testFilesDirectory, category, "*.json"))
			require.NoError(t, err)

			for _, testFile := range testFiles {
				testName := strings.TrimSuffix(filepath.Base(testFile), ".json")

				t.Run(testName, func(t *testing.T) {
					t.Parallel()

					observedZapCore, observedLogs := observer.New(zap.WarnLevel)
					unmarshaler := NewAzureResourceLogsUnmarshaler(
						testBuildInfo,
						zap.New(observedZapCore),
						LogsConfig{
							TimeFormats: []string{
								"01/02/2006 15:04:05",
								"2006-01-02T15:04:05Z",
								"1/2/2006 3:04:05.000 PM -07:00",
								"1/2/2006 3:04:05 PM -07:00",
							},
						},
					)

					data, err := os.ReadFile(testFile)
					require.NoError(t, err)

					logs, err := unmarshaler.UnmarshalLogs(data)
					require.NoError(t, err)

					require.Equal(t, 0, observedLogs.Len(), "Unexpected error/warn logs: %+v", observedLogs.All())

					expectedLogs, err := golden.ReadLogs(filepath.Join(testFilesDirectory, category, fmt.Sprintf("%s_expected.yaml", testName)))
					require.NoError(t, err)

					compareOptions := []plogtest.CompareLogsOption{
						plogtest.IgnoreResourceLogsOrder(),
						plogtest.IgnoreLogRecordsOrder(),
					}
					require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, compareOptions...))
				})
			}
		})
	}
}

func TestResourceLogsUnmarshaler_UnmarshalLogs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		cfg                LogsConfig
		data               []byte
		expectedError      string
		expectedLogLine    string
		expectedLogRecords int
	}{
		{
			name:               "Empty Records",
			cfg:                LogsConfig{},
			data:               []byte(`{"records": []}`),
			expectedLogRecords: 0,
		},
		{
			name:               "Unsupported JSON Format",
			cfg:                LogsConfig{},
			data:               []byte(`{"other-root": []}`),
			expectedLogRecords: 0,
			expectedError:      "unable to detect JSON format from input",
		},
		{
			name:               "Invalid JSON",
			cfg:                LogsConfig{},
			data:               []byte(`{invalid-json}`),
			expectedLogRecords: 0,
			expectedError:      "unable to detect JSON format from input",
		},
		{
			name: "Records Format",
			cfg:  LogsConfig{},
			data: []byte(`{
				"records": [
					{
						"time": "2022-11-11T04:48:27.6767145Z",
						"resourceId": "/RESOURCE_ID",
						"operationName": "SecretGet",
						"category": "AuditEvent"
					},
					{
						"time": "2022-11-12T04:48:27.6767145Z",
						"resourceId": "/RESOURCE_ID2",
						"operationName": "SecretGet",
						"category": "AuditEvent"
					}
				]
			}`),
			expectedLogRecords: 2,
		},
		{
			name: "JSON Array Format",
			cfg:  LogsConfig{},
			data: []byte(`[
					{
						"time": "2022-11-11T04:48:27.6767145Z",
						"resourceId": "/RESOURCE_ID",
						"operationName": "SecretGet",
						"category": "AuditEvent"
					},
					{
						"time": "2022-11-12T04:48:27.6767145Z",
						"resourceId": "/RESOURCE_ID2",
						"operationName": "SecretGet",
						"category": "AuditEvent"
					}
				]`),
			expectedLogRecords: 2,
		},
		{
			name: "ND JSON Format",
			cfg:  LogsConfig{},
			data: []byte(`{"time":"2022-11-11T04:48:27.6767145Z","resourceId":"/RESOURCE_ID","operationName":"SecretGet","category":"AuditEvent"}
{"time":"2022-11-12T04:48:27.6767145Z","resourceId":"/RESOURCE_ID2","operationName":"SecretGet","category":"AuditEvent"}`),
			expectedLogRecords: 2,
		},
		{
			name: "Empty Category",
			cfg:  LogsConfig{},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "",
					"type": ""
				}]
			}`),
			expectedLogRecords: 1,
		},
		{
			name: "Category from Type field",
			cfg:  LogsConfig{},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"type": "AuditEvent"
				}]
			}`),
			expectedLogRecords: 1,
		},
		{
			name: "Empty ResourceID",
			cfg:  LogsConfig{},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "",
					"operationName": "SecretGet",
					"category": "AuditEvent"
				}]
			}`),
			expectedLogRecords: 1,
			expectedLogLine:    "No ResourceID set on Log record",
		},
		{
			name: "No Timestamp",
			cfg:  LogsConfig{},
			data: []byte(`{
				"records": [{
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "AuditEvent"
				}]
			}`),
			expectedLogRecords: 1,
			expectedLogLine:    "Unable to convert timestamp from log",
		},
		{
			name: "Invalid Timestamp",
			cfg:  LogsConfig{},
			data: []byte(`{
				"records": [{
					"time": "invalid-timestamp",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "AuditEvent"
				}]
			}`),
			expectedLogRecords: 1,
			expectedLogLine:    "Unable to convert timestamp from log",
		},
		{
			name: "Exclude Category Filter",
			cfg: LogsConfig{
				ExcludeCategories: []string{"AuditEvent"},
			},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "AuditEvent"
				}]
			}`),
			expectedLogRecords: 0,
		},
		{
			name: "Exclude Category Filter No Match",
			cfg: LogsConfig{
				ExcludeCategories: []string{"ApplicationGatewayAccess"},
			},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "AuditEvent"
				}]
			}`),
			expectedLogRecords: 1,
		},
		{
			name: "Include Category Filter",
			cfg: LogsConfig{
				IncludeCategories: []string{"ApplicationGatewayAccess"},
			},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "ApplicationGatewayAccess"
				}]
			}`),
			expectedLogRecords: 1,
		},
		{
			name: "Include Category Filter No Match",
			cfg: LogsConfig{
				IncludeCategories: []string{"ApplicationGatewayAccess"},
			},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "AuditEvent"
				}]
			}`),
			expectedLogRecords: 0,
		},
		{
			name: "Exclude Category Filter Priority Over Include",
			cfg: LogsConfig{
				ExcludeCategories: []string{"ApplicationGatewayAccess"},
				IncludeCategories: []string{"ApplicationGatewayAccess"},
			},
			data: []byte(`{
				"records": [{
					"time": "2022-11-11T04:48:27.6767145Z",
					"resourceId": "/RESOURCE_ID",
					"operationName": "SecretGet",
					"category": "ApplicationGatewayAccess"
				}]
			}`),
			expectedLogRecords: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedZapCore, observedLogs := observer.New(zap.WarnLevel)
			unmarshaler := NewAzureResourceLogsUnmarshaler(
				testBuildInfo,
				zap.New(observedZapCore),
				tt.cfg,
			)
			logs, err := unmarshaler.UnmarshalLogs(tt.data)
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
			if tt.expectedLogLine != "" {
				require.Equal(t, 1, observedLogs.FilterMessage(tt.expectedLogLine).Len())
			}
			require.Equal(t, tt.expectedLogRecords, logs.LogRecordCount())
		})
	}
}
