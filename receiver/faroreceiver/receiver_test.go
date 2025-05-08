// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver/internal/metadata"
)

func TestFaroReceiver_Start(t *testing.T) {
	testcases := []struct {
		name               string
		payload            string
		expectedStatusCode int
		expectedTraces     string
		expectedLogs       string
	}{
		{
			name:               "empty",
			payload:            filepath.Join("testdata", "empty.json"),
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name:               "minimal-traces-only",
			payload:            filepath.Join("testdata", "traces", "minimal-traces-only.json"),
			expectedStatusCode: http.StatusOK,
			expectedTraces:     filepath.Join("testdata", "golden", "minimal-traces-only.yaml"),
		},
		{
			name:               "minimal-logs-only",
			payload:            filepath.Join("testdata", "logs", "minimal-logs-only.json"),
			expectedStatusCode: http.StatusOK,
			expectedLogs:       filepath.Join("testdata", "golden", "minimal-logs-only.yaml"),
		},
		{
			name:               "minimal-logs-and-traces-only",
			payload:            filepath.Join("testdata", "logsandtraces", "minimal-only.json"),
			expectedStatusCode: http.StatusOK,
			expectedLogs:       filepath.Join("testdata", "golden", "minimal-logs-only.yaml"),
			expectedTraces:     filepath.Join("testdata", "golden", "minimal-traces-only.yaml"),
		},
	}

	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = zap.NewNop()

	receiver, err := newFaroReceiver(cfg, &settings)
	require.NoError(t, err)

	nextTraces := new(consumertest.TracesSink)
	nextLogs := new(consumertest.LogsSink)
	receiver.RegisterTracesConsumer(nextTraces)
	receiver.RegisterLogsConsumer(nextLogs)

	err = receiver.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { require.NoError(t, receiver.Shutdown(context.Background())) }()

	server := httptest.NewServer(http.HandlerFunc(receiver.handleFaroRequest))
	defer server.Close()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			nextTraces.Reset()
			nextLogs.Reset()

			contents, err := os.ReadFile(tc.payload)
			require.NoError(t, err)

			req, err := http.NewRequest(http.MethodPost, server.URL+faroPath, bytes.NewBuffer(contents))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatusCode, resp.StatusCode)

			if tc.expectedTraces != "" {
				traces := nextTraces.AllTraces()
				require.Len(t, traces, 1)
				expected, err := golden.ReadTraces(tc.expectedTraces)
				require.NoError(t, err)
				require.NoError(t, ptracetest.CompareTraces(expected, traces[0]))
			}
			if tc.expectedLogs != "" {
				logs := nextLogs.AllLogs()
				require.Len(t, logs, 1)
				expected, err := golden.ReadLogs(tc.expectedLogs)
				require.NoError(t, err)
				require.NoError(t, plogtest.CompareLogs(expected, logs[0]))
			}
		})
	}
}
