// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

const testFilesDirectory = "testdata"

func TestResourceTracesUnmarshaler_UnmarshalTraces_Golden(t *testing.T) {
	t.Parallel()

	testFiles, err := filepath.Glob(filepath.Join(testFilesDirectory, "*.json"))
	require.NoError(t, err)

	for _, testFile := range testFiles {
		testName := strings.TrimSuffix(filepath.Base(testFile), ".json")

		t.Run(testName, func(t *testing.T) {
			t.Parallel()
			unmarshaler := NewAzureResourceTracesUnmarshaler(
				component.BuildInfo{Version: "test-version"},
				zap.NewNop(),
				TracesConfig{},
			)

			data, err := os.ReadFile(filepath.Join(testFilesDirectory, fmt.Sprintf("%s.json", testName)))
			require.NoError(t, err)

			traces, err := unmarshaler.UnmarshalTraces(data)
			require.NoError(t, err)

			expectedMetrics, err := golden.ReadTraces(filepath.Join(testFilesDirectory, fmt.Sprintf("%s_expected.yaml", testName)))
			require.NoError(t, err)

			compareOptions := []ptracetest.CompareTracesOption{
				ptracetest.IgnoreResourceSpansOrder(),
				ptracetest.IgnoreSpansOrder(),
			}

			require.NoError(t, ptracetest.CompareTraces(expectedMetrics, traces, compareOptions...))
		})
	}
}

func TestResourceTracesUnmarshaler_UnmarshalTraces(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		cfg             TracesConfig
		data            []byte
		expectedError   string
		expectedLogLine string
		expectedSpans   int
	}{
		{
			name:          "Empty Records",
			cfg:           TracesConfig{},
			data:          []byte(`{"records": []}`),
			expectedSpans: 0,
		},
		{
			name:          "Invalid JSON",
			cfg:           TracesConfig{},
			data:          []byte(`{invalid-json}`),
			expectedSpans: 0,
			expectedError: "unable to detect JSON format from input",
		},
		{
			name: "Empty ResourceID",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "",
					"Type": "AppDependencies",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "4821911c2c909251",
					"ParentId": "56d86b8fef4f1c6c"
				}]
			}`),
			expectedSpans:   1,
			expectedLogLine: "No ResourceID set on Trace record",
		},
		{
			name: "Invalid Category",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "UnknownCategory",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "4821911c2c909251",
					"ParentId": "56d86b8fef4f1c6c"
				}]
			}`),
			expectedSpans: 0,
		},
		{
			name: "Invalid Timestamp",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "invalid-timestamp",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "AppDependencies",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "4821911c2c909251",
					"ParentId": "56d86b8fef4f1c6c"
				}]
			}`),
			expectedSpans:   0,
			expectedLogLine: "Unable to convert timestamp from Trace Record",
		},
		{
			name: "Invalid TraceID",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "AppDependencies",
					"OperationId": "not-a-trace-id",
					"Id": "4821911c2c909251",
					"ParentId": "56d86b8fef4f1c6c"
				}]
			}`),
			expectedSpans:   0,
			expectedLogLine: "Unable to convert TraceID from Trace Record",
		},
		{
			name: "Invalid SpanID",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "AppDependencies",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "not-a-span-id",
					"ParentId": "56d86b8fef4f1c6c"
				}]
			}`),
			expectedSpans:   0,
			expectedLogLine: "Unable to convert SpanID from Trace Record",
		},
		{
			name: "SpanID equals to TraceID",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "AppDependencies",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "28aad6f31b49a543549b6721bde89c81",
					"ParentId": "56d86b8fef4f1c6c"
				}]
			}`),
			expectedSpans: 1,
		},
		{
			name: "Invalid Parent SpanID",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "AppDependencies",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "4821911c2c909251",
					"ParentId": "not-a-span-id"
				}]
			}`),
			expectedSpans:   1,
			expectedLogLine: "Unable to convert ParentSpanID from Trace Record",
		},
		{
			name: "Parent SpanID equals to TraceID",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "AppDependencies",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "28aad6f31b49a543549b6721bde89c81",
					"ParentId": "28aad6f31b49a543549b6721bde89c81"
				}]
			}`),
			expectedSpans: 1,
		},
		{
			name: "Empty Parent SpanID",
			cfg:  TracesConfig{},
			data: []byte(`{
				"records": [{
					"time": "2023-04-18T09:03:00.0000000Z",
					"resourceId": "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.INSIGHTS/COMPONENTS/APPI1",
					"Type": "AppDependencies",
					"OperationId": "28aad6f31b49a543549b6721bde89c81",
					"Id": "4821911c2c909251",
					"ParentId": "56d86b8fef4f1c6c"
				}]
			}`),
			expectedSpans: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedZapCore, observedLogs := observer.New(zap.WarnLevel)
			unmarshaler := NewAzureResourceTracesUnmarshaler(
				component.BuildInfo{Version: "test-version"},
				zap.New(observedZapCore),
				TracesConfig{},
			)
			traces, err := unmarshaler.UnmarshalTraces(tt.data)
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
			if tt.expectedLogLine != "" {
				require.Equal(t, 1, observedLogs.FilterMessage(tt.expectedLogLine).Len())
			}
			require.Equal(t, tt.expectedSpans, traces.SpanCount())
		})
	}
}
