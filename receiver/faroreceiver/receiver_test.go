// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faroreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver"

import (
	"bytes"
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
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver/internal/metadatatest"
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
			expectedStatusCode: http.StatusAccepted,
			expectedTraces:     filepath.Join("testdata", "golden", "minimal-traces-only.yaml"),
		},
		{
			name:               "minimal-logs-only",
			payload:            filepath.Join("testdata", "logs", "minimal-logs-only.json"),
			expectedStatusCode: http.StatusAccepted,
			expectedLogs:       filepath.Join("testdata", "golden", "minimal-logs-only.yaml"),
		},
		{
			name:               "minimal-logs-and-traces-only",
			payload:            filepath.Join("testdata", "logsandtraces", "minimal-only.json"),
			expectedStatusCode: http.StatusAccepted,
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

	err = receiver.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { require.NoError(t, receiver.Shutdown(t.Context())) }()

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
				require.NoError(t, plogtest.CompareLogs(expected, logs[0], plogtest.IgnoreObservedTimestamp()))
			}
		})
	}
}

func TestFaroReceiver_CustomTelemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	defer func() { require.NoError(t, tel.Shutdown(t.Context())) }()

	cfg := createDefaultConfig().(*Config)
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.TelemetrySettings = tel.NewTelemetrySettings()
	settings.Logger = zap.NewNop()

	receiver, err := newFaroReceiver(cfg, &settings)
	require.NoError(t, err)

	receiver.RegisterTracesConsumer(new(consumertest.TracesSink))
	receiver.RegisterLogsConsumer(new(consumertest.LogsSink))

	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, receiver.Shutdown(t.Context())) }()

	server := httptest.NewServer(http.HandlerFunc(receiver.handleFaroRequest))
	defer server.Close()

	contents, err := os.ReadFile(filepath.Join("testdata", "all-signals.json"))
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, server.URL+faroPath, bytes.NewBuffer(contents))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	metadatatest.AssertEqualFaroLogIngested(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 2}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualFaroMeasurementIngested(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 3}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualFaroExceptionIngested(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualFaroEventIngested(t, tel,
		[]metricdata.DataPoint[int64]{{Value: 4}},
		metricdatatest.IgnoreTimestamp())
}
