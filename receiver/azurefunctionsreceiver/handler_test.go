// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurefunctionsreceiver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/eventhub"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/handler"
	invokeprotocol "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/protocol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/transport"
)

func TestHandleLogs(t *testing.T) {
	tests := map[string]struct {
		testDataFile           string
		expectedLogs           int
		expectedReturnValue    string
		expectedErrorSubstring string
		checkMetadata          bool
	}{
		"valid_logs": {
			testDataFile:        "valid_logs.request.json",
			expectedLogs:        1,
			expectedReturnValue: "success",
			checkMetadata:       true,
		},
		"invalid_method": {
			testDataFile:           "invalid_method.request.json",
			expectedLogs:           0,
			expectedReturnValue:    "failure",
			expectedErrorSubstring: "missing data for binding",
		},
		"invalid_request": {
			testDataFile:           "invalid_request.json.txt",
			expectedLogs:           0,
			expectedReturnValue:    "failure",
			expectedErrorSubstring: "decode invoke request",
		},
		"invalid_logs": {
			testDataFile:           "invalid_logs.request.json",
			expectedLogs:           0,
			expectedReturnValue:    "failure",
			expectedErrorSubstring: "unmarshal message 0",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logsSink := consumertest.LogsSink{}
			unmarshaler := &testLogsUnmarshaler{} // returns one log for "valid", error for "invalid"
			decoder := transport.NewBinaryDecoder()
			protocol := NewInvokeProtocol(decoder, zap.NewNop(), eventhub.ExtractMetadata)
			consumer := eventhub.NewLogsConsumer(unmarshaler, &logsSink)
			profile := NewProfile("logs", protocol, consumer)
			h := createHandler(profile)

			requestBody, err := os.ReadFile(filepath.Join("testdata", test.testDataFile))
			require.NoError(t, err, "failed to read test data file")

			// Create request with body from file
			req := httptest.NewRequest(http.MethodPost, "/logs", io.NopCloser(bytes.NewReader(requestBody)))
			// Create response recorder
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Result().StatusCode, "status code")
			require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"), "content-type")

			body, err := io.ReadAll(w.Result().Body)
			require.NoError(t, err, "failed to read response body")

			var resp invokeprotocol.InvokeResponse
			require.NoError(t, json.Unmarshal(body, &resp))
			assert.Equal(t, test.expectedReturnValue, resp.ReturnValue, "returnValue")

			if test.expectedErrorSubstring != "" {
				require.NotNil(t, resp.Outputs, "outputs should be set on failure")
				assert.Contains(t, resp.Outputs.FailedMessage.Error, test.expectedErrorSubstring, "error message")
			}

			assert.Equal(t, test.expectedLogs, len(logsSink.AllLogs()), "number of log batches")

			if test.checkMetadata && test.expectedLogs > 0 {
				for _, logs := range logsSink.AllLogs() {
					for i := 0; i < logs.ResourceLogs().Len(); i++ {
						resource := logs.ResourceLogs().At(i).Resource()
						name, ok := resource.Attributes().Get(eventhub.AttrEventHubName)
						require.True(t, ok)
						assert.Equal(t, "logs", name.Str())
						partitionID, ok := resource.Attributes().Get(eventhub.AttrEventHubPartitionID)
						require.True(t, ok)
						assert.Equal(t, "3", partitionID.Str())
						consumerGroup, ok := resource.Attributes().Get(eventhub.AttrEventHubConsumerGroup)
						require.True(t, ok)
						assert.Equal(t, "test", consumerGroup.Str())
						namespace, ok := resource.Attributes().Get(eventhub.AttrEventHubNamespace)
						require.True(t, ok)
						assert.Equal(t, "test-namespace.servicebus.windows.net", namespace.Str())
					}
				}
			}

			logsSink.Reset()
		})
	}
}

// testLogsUnmarshaler returns one log record for payload "valid", error for "invalid".
type testLogsUnmarshaler struct{}

func (testLogsUnmarshaler) UnmarshalLogs(data []byte) (plog.Logs, error) {
	if string(data) == "invalid" {
		return plog.Logs{}, errors.New("invalid log payload")
	}
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty()
	return logs, nil
}

// --- Unit tests for invoke protocol and handler edge cases ---

type errReader struct{}

func (errReader) Read(_ []byte) (n int, err error) {
	return 0, errors.New("read failed")
}

func TestCreateHandler_ReadBodyError(t *testing.T) {
	mockConsumer := &mockConsumer{}
	protocol := NewInvokeProtocol(transport.NewBinaryDecoder(), zap.NewNop(), nil)
	profile := NewProfile("logs", protocol, mockConsumer)
	h := createHandler(profile)

	req := httptest.NewRequest(http.MethodPost, "/logs", nil)
	req.Body = io.NopCloser(errReader{})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.False(t, mockConsumer.consumed)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	body, _ := io.ReadAll(w.Result().Body)
	var resp invokeprotocol.InvokeResponse
	require.NoError(t, json.Unmarshal(body, &resp))
	assert.Equal(t, "failure", resp.ReturnValue)
	assert.Contains(t, resp.Outputs.FailedMessage.Error, "read body")
	assert.Nil(t, resp.Outputs.FailedMessage.Source)
}

type mockConsumer struct {
	consumed    bool
	consumeFunc func(context.Context, handler.ParsedRequest) error
}

func (m *mockConsumer) ConsumeEvents(ctx context.Context, req handler.ParsedRequest) error {
	m.consumed = true
	if m.consumeFunc != nil {
		return m.consumeFunc(ctx, req)
	}
	return nil
}
