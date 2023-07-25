// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v3"
)

func TestOpenSearchExporter(t *testing.T) {
	// Create HTTP listener
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		decoder := json.NewDecoder(r.Body)
		for decoder.More() {
			var jsonData any
			err = decoder.Decode(&jsonData)
			require.NoError(t, err)
			require.NotNil(t, jsonData)

			strMap := jsonData.(map[string]any)
			if actionData, isBulkAction := strMap["create"]; isBulkAction {
				validateBulkAction(t, actionData.(map[string]any))
			} else {
				validateTraceJSON(t, strMap)
			}
		}
	}))
	defer ts.Close()

	// Create exporter
	f := NewFactory()
	cfg := withDefaultConfig(func(config *Config) {
		config.Endpoint = ts.URL
	})
	exporter, err := f.CreateTracesExporter(context.Background(), exportertest.NewNopCreateSettings(), cfg)
	require.NoError(t, err)

	// Initialize the exporter
	err = exporter.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Load sample data
	traces, err := readTraces("testdata/traces.yaml")
	require.NoError(t, err)

	// Send it
	err = exporter.ConsumeTraces(context.Background(), traces)
	require.NoError(t, err)
}

func validateTraceJSON(t *testing.T, strMap map[string]any) {
	require.NotEmpty(t, strMap)
	// TODO would be excellent place for schema validation once the schema is published
}

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
