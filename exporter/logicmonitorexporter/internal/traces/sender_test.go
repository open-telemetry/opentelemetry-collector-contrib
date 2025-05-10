// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	lmsdktraces "github.com/logicmonitor/lm-data-sdk-go/api/traces"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.uber.org/zap"
)

func TestSendTraces(t *testing.T) {
	authParams := utils.AuthParams{
		AccessID:    "testId",
		AccessKey:   "testKey",
		BearerToken: "testToken",
	}
	t.Run("should not return error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			response := lmsdktraces.LMTraceIngestResponse{
				Success: true,
				Message: "Accepted",
			}
			w.WriteHeader(http.StatusAccepted)
			assert.NoError(t, json.NewEncoder(w).Encode(&response))
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(context.Background())

		sender, err := NewSender(ctx, ts.URL, ts.Client(), authParams, zap.NewNop())
		assert.NoError(t, err)

		err = sender.SendTraces(ctx, testdata.GenerateTraces(1))
		cancel()
		assert.NoError(t, err)
	})

	t.Run("should return permanent failure error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			response := lmsdktraces.LMTraceIngestResponse{
				Success: false,
				Message: "The request is invalid. For example, it may be missing headers or the request body is incorrectly formatted.",
			}
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(&response))
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(context.Background())

		sender, err := NewSender(ctx, ts.URL, ts.Client(), authParams, zap.NewNop())
		assert.NoError(t, err)

		err = sender.SendTraces(ctx, testdata.GenerateTraces(1))
		cancel()
		assert.Error(t, err)
		assert.True(t, consumererror.IsPermanent(err))
	})

	t.Run("should not return permanent failure error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			response := lmsdktraces.LMTraceIngestResponse{
				Success: false,
				Message: "A dependency failed to respond within a reasonable time.",
			}
			w.WriteHeader(http.StatusBadGateway)
			assert.NoError(t, json.NewEncoder(w).Encode(&response))
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(context.Background())

		sender, err := NewSender(ctx, ts.URL, ts.Client(), authParams, zap.NewNop())
		assert.NoError(t, err)

		err = sender.SendTraces(ctx, testdata.GenerateTraces(1))
		cancel()
		assert.Error(t, err)
		assert.False(t, consumererror.IsPermanent(err))
	})
}
