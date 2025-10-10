// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	lmsdklogs "github.com/logicmonitor/lm-data-sdk-go/api/logs"
	"github.com/logicmonitor/lm-data-sdk-go/model"
	"github.com/logicmonitor/lm-data-sdk-go/utils"
	"github.com/logicmonitor/lm-data-sdk-go/utils/translator"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"
)

func TestSendLogs(t *testing.T) {
	t.Run("should not return error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			response := lmsdklogs.LMLogIngestResponse{
				Success: true,
				Message: "Accepted",
			}
			w.WriteHeader(http.StatusAccepted)
			assert.NoError(t, json.NewEncoder(w).Encode(&response))
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(t.Context())

		sender, err := NewSender(ctx, zap.NewNop(), buildLogIngestTestOpts(ts.URL, ts.Client())...)
		assert.NoError(t, err)

		logInput := translator.ConvertToLMLogInput("test msg", "ERROR", utils.NewTimestampFromTime(time.Now()).String(), map[string]any{"system.hostname": "test"}, map[string]any{"cloud.provider": "aws"})
		err = sender.SendLogs(ctx, []model.LogInput{logInput})
		cancel()
		assert.NoError(t, err)
	})

	t.Run("should return permanent failure error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			response := lmsdklogs.LMLogIngestResponse{
				Success: false,
				Message: "The request is invalid. For example, it may be missing headers or the request body is incorrectly formatted.",
			}
			w.WriteHeader(http.StatusBadRequest)
			assert.NoError(t, json.NewEncoder(w).Encode(&response))
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(t.Context())

		sender, err := NewSender(ctx, zap.NewNop(), buildLogIngestTestOpts(ts.URL, ts.Client())...)
		assert.NoError(t, err)

		logInput := translator.ConvertToLMLogInput("test msg", "INFO", utils.NewTimestampFromTime(time.Now()).String(), map[string]any{"system.hostname": "test"}, map[string]any{"cloud.provider": "aws"})
		err = sender.SendLogs(ctx, []model.LogInput{logInput})
		cancel()
		assert.Error(t, err)
		assert.True(t, consumererror.IsPermanent(err))
	})

	t.Run("should not return permanent failure error", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			response := lmsdklogs.LMLogIngestResponse{
				Success: false,
				Message: "A dependency failed to respond within a reasonable time.",
			}
			w.WriteHeader(http.StatusBadGateway)
			assert.NoError(t, json.NewEncoder(w).Encode(&response))
		}))
		defer ts.Close()

		ctx, cancel := context.WithCancel(t.Context())

		sender, err := NewSender(ctx, zap.NewNop(), buildLogIngestTestOpts(ts.URL, ts.Client())...)
		assert.NoError(t, err)

		logInput := translator.ConvertToLMLogInput("test msg", "DEBUG", utils.NewTimestampFromTime(time.Now()).String(), map[string]any{"system.hostname": "test"}, map[string]any{"cloud.provider": "aws"})
		err = sender.SendLogs(ctx, []model.LogInput{logInput})
		cancel()
		assert.Error(t, err)
		assert.False(t, consumererror.IsPermanent(err))
	})
}

func buildLogIngestTestOpts(endpoint string, client *http.Client) []lmsdklogs.Option {
	authParams := utils.AuthParams{
		AccessID:    "testId",
		AccessKey:   "testKey",
		BearerToken: "testToken",
	}

	opts := []lmsdklogs.Option{
		lmsdklogs.WithLogBatchingDisabled(),
		lmsdklogs.WithAuthentication(authParams),
		lmsdklogs.WithHTTPClient(client),
		lmsdklogs.WithEndpoint(endpoint),
	}
	return opts
}
