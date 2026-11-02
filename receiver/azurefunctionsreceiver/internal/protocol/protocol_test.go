// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInvokeRequest(t *testing.T) {
	t.Run("valid_minimal", func(t *testing.T) {
		body := []byte(`{"Data":{"logs":"[\"dGVzdA==\"]"},"Metadata":{"TriggerPartitionContext":{"FullyQualifiedNamespace":"ns.servicebus.windows.net","EventHubName":"logs","ConsumerGroup":"$Default","PartitionId":"0"},"sys":{"MethodName":"logs"}}}`)
		req, err := ParseInvokeRequest(body)
		require.NoError(t, err)
		assert.NotNil(t, req.Data)
		assert.Equal(t, "[\"dGVzdA==\"]", req.Data["logs"])
		assert.NotEmpty(t, req.Metadata, "Metadata should be passed through as raw JSON for trigger-specific parsing")
	})

	t.Run("invalid_json", func(t *testing.T) {
		_, err := ParseInvokeRequest([]byte("not json"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "decode invoke request")
	})

	t.Run("empty_body", func(t *testing.T) {
		req, err := ParseInvokeRequest([]byte("{}"))
		require.NoError(t, err)
		assert.Nil(t, req.Data)
	})
}

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	WriteSuccess(w)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var resp InvokeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "success", resp.ReturnValue)
	assert.Nil(t, resp.Outputs)
}

func TestWriteFailure(t *testing.T) {
	w := httptest.NewRecorder()
	body := []byte("original request body")
	err := errors.New("test error")
	WriteFailure(w, err, body)
	assert.Equal(t, http.StatusOK, w.Code) // Azure protocol uses 200 with returnValue failure
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	var resp InvokeResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "failure", resp.ReturnValue)
	require.NotNil(t, resp.Outputs)
	assert.Equal(t, "test error", resp.Outputs.FailedMessage.Error)
	assert.Equal(t, body, resp.Outputs.FailedMessage.Source)
}
