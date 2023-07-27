// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v3"
)

func TestOpenSearchExporter(t *testing.T) {
	type requestHandler struct {
		ValidateReceivedDocuments func(*testing.T, int, []map[string]any)
		ResponseJSONPath          string
	}

	sanityCheckAndRespond := func(responsePath string) requestHandler {
		pass := func(t *testing.T, _ int, docs []map[string]any) {
			for _, doc := range docs {
				require.NotEmpty(t, doc)
			}
		}
		return requestHandler{pass, responsePath}
	}
	tests := []struct {
		Label                  string
		TracePath              string
		RequestHandlers        []requestHandler
		ValidateExporterReturn func(error)
	}{
		{
			"Round trip",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				sanityCheckAndRespond("testdata/opensearch-response-no-error.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},
		{
			"Permanent error",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				sanityCheckAndRespond("testdata/opensearch-response-permanent-error.json"),
			},
			func(err error) {
				require.True(t, consumererror.IsPermanent(err))
			},
		},
		{
			"Retryable error",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				sanityCheckAndRespond("testdata/opensearch-response-retryable-error.json"),
				sanityCheckAndRespond("testdata/opensearch-response-retryable-succeeded.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},

		{
			"Retryable error, succeeds on second try",
			"testdata/traces-sample-a.yaml",
			[]requestHandler{
				sanityCheckAndRespond("testdata/opensearch-response-retryable-error.json"),
				sanityCheckAndRespond("testdata/opensearch-response-retryable-error-2-attempt.json"),
				sanityCheckAndRespond("testdata/opensearch-response-retryable-succeeded.json"),
			},
			func(err error) {
				require.NoError(t, err)
			},
		},
	}

	getReceivedDocuments := func(body io.ReadCloser) []map[string]any {
		var rtn []map[string]any
		var err error
		decoder := json.NewDecoder(body)
		for decoder.More() {
			var jsonData any
			err = decoder.Decode(&jsonData)
			require.NoError(t, err)
			require.NotNil(t, jsonData)

			strMap := jsonData.(map[string]any)
			if actionData, isBulkAction := strMap["create"]; isBulkAction {
				validateBulkAction(t, actionData.(map[string]any))
			} else {
				rtn = append(rtn, strMap)
			}
		}
		return rtn
	}

	for _, tc := range tests {
		// Create HTTP listener
		var requestCount = 0
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			docs := getReceivedDocuments(r.Body)
			require.LessOrEqualf(t, requestCount, len(tc.RequestHandlers), "Test case generated more requests than it has response for.")
			tc.RequestHandlers[requestCount].ValidateReceivedDocuments(t, requestCount, docs)

			w.WriteHeader(200)
			var nextResponseDoc = requestCount % len(tc.RequestHandlers)
			response, _ := os.ReadFile(tc.RequestHandlers[nextResponseDoc].ResponseJSONPath)
			_, err = w.Write(response)
			require.NoError(t, err)

			requestCount++
		}))

		cfg := withDefaultConfig(func(config *Config) {
			config.Endpoint = ts.URL
			config.TimeoutSettings.Timeout = 0
		})

		// Create exporter
		f := NewFactory()
		exporter, err := f.CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
		require.NoError(t, err)

		// Initialize the exporter
		err = exporter.Start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)

		// Load sample data
		traces, err := readTraces(tc.TracePath)
		require.NoError(t, err)

		// Send it
		err = exporter.ConsumeTraces(context.Background(), traces)
		tc.ValidateExporterReturn(err)
		err = exporter.Shutdown(context.Background())
		require.NoError(t, err)
		ts.Close()
	}
}

// validateBulkAction ensures  the action JSON object is to the correct index.
func validateBulkAction(t *testing.T, strMap map[string]any) {
	val, exists := strMap["_index"]
	require.True(t, exists)
	require.Equal(t, val, "sso_traces-default-namespace")
}

// readTraces loads a yaml file at given filePath and converts the content to ptrace.Traces
func readTraces(filePath string) (ptrace.Traces, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return ptrace.Traces{}, err
	}
	var m map[string]interface{}
	if err = yaml.Unmarshal(b, &m); err != nil {
		return ptrace.Traces{}, err
	}
	b, err = json.Marshal(m)
	if err != nil {
		return ptrace.Traces{}, err
	}
	unmarshaler := ptrace.JSONUnmarshaler{}
	return unmarshaler.UnmarshalTraces(b)
}
