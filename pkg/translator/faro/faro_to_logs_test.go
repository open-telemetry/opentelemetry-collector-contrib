// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestTranslateToLogs(t *testing.T) {
	testcases := []struct {
		name             string
		faroPayload      faroTypes.Payload
		expectedLogsFile string
		wantErr          assert.ErrorAssertionFunc
	}{
		{
			name:             "Empty payload",
			faroPayload:      faroTypes.Payload{},
			expectedLogsFile: filepath.Join("testdata", "empty-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Standard payload",
			faroPayload:      PayloadFromFile(t, "standard-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "standard-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with browser brands as slice",
			faroPayload:      PayloadFromFile(t, "browser-brand-slice-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "browser-brand-slice-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with browser brands as string",
			faroPayload:      PayloadFromFile(t, "browser-brand-string-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "browser-brand-string-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
		{
			name:             "Payload with actions",
			faroPayload:      PayloadFromFile(t, "actions-payload/payload.json"),
			expectedLogsFile: filepath.Join("testdata", "actions-payload", "plogs.yaml"),
			wantErr:          assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			actualLogs, err := TranslateToLogs(context.TODO(), tt.faroPayload)
			if !tt.wantErr(t, err) {
				return
			}
			expectedLogs, err := golden.ReadLogs(tt.expectedLogsFile)
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs))
		})
	}
}

func TestTranslateFromFaroToOTLPAndBack(t *testing.T) {
	faroPayload := PayloadFromFile(t, "general/payload.json")
	// Translate from faro payload to otlp logs
	actualLogs, err := TranslateToLogs(context.TODO(), faroPayload)
	require.NoError(t, err)

	// Translate from faro payload to otlp traces
	actualTraces, err := TranslateToTraces(context.TODO(), faroPayload)
	require.NoError(t, err)

	// Translate from otlp logs to faro payload
	faroLogsPayloads, err := TranslateFromLogs(context.TODO(), actualLogs)
	require.NoError(t, err)

	// Translate from otlp traces to faro payload
	faroTracesPayloads, err := TranslateFromTraces(context.TODO(), actualTraces)
	require.NoError(t, err)

	// Combine the traces and logs faro payloads into a single faro payload
	expectedFaroPayload := faroLogsPayloads[0]
	expectedFaroPayload.Traces = faroTracesPayloads[0].Traces

	// Compare the original faro payload with the translated faro payload
	require.Equal(t, expectedFaroPayload, faroPayload)
}

func TestTranslateFromOTLPToFaroAndBack(t *testing.T) {
	logs, err := golden.ReadLogs(filepath.Join("testdata", "general", "plogs.yaml"))
	require.NoError(t, err)

	traces, err := golden.ReadTraces(filepath.Join("testdata", "general", "ptraces.yaml"))
	require.NoError(t, err)

	// Translate from otlp logs to faro payload
	actualFaroLogsPayloads, err := TranslateFromLogs(context.TODO(), logs)
	require.NoError(t, err)

	// Translate from otlp traces to faro payload
	actualFaroTracesPayloads, err := TranslateFromTraces(context.TODO(), traces)
	require.NoError(t, err)

	// Combine the traces and logs faro payloads into a single faro payload
	actualFaroPayload := actualFaroLogsPayloads[0]
	actualFaroPayload.Traces = actualFaroTracesPayloads[0].Traces

	// Translate from faro payload to otlp logs
	expectedLogs, err := TranslateToLogs(context.TODO(), actualFaroPayload)
	require.NoError(t, err)

	// Compare the original otlp logs with the translated otlp logs
	require.Equal(t, expectedLogs, logs)

	// Translate from faro payload to otlp traces
	expectedTraces, err := TranslateToTraces(context.TODO(), actualFaroPayload)
	require.NoError(t, err)

	// Compare the original otlp traces with the translated otlp traces
	require.Equal(t, expectedTraces, traces)
}

func PayloadFromFile(t *testing.T, filename string) faroTypes.Payload {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	var p faroTypes.Payload
	if err := json.NewDecoder(f).Decode(&p); err != nil {
		t.Fatal(err)
	}

	return p
}
