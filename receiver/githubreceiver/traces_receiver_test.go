// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestHealthCheck(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	defaultConfig.WebHook.Endpoint = "localhost:0"
	consumer := consumertest.NewNop()
	receiver, err := newTracesReceiver(receivertest.NewNopSettings(), defaultConfig, consumer)
	require.NoError(t, err, "failed to create receiver")

	r := receiver
	require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()), "failed to start receiver")
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()), "failed to shutdown revceiver")
	}()

	w := httptest.NewRecorder()
	r.handleHealthCheck(w, httptest.NewRequest(http.MethodGet, "http://localhost/health", nil))

	response := w.Result()
	require.Equal(t, http.StatusOK, response.StatusCode)
}

func TestWebhook(t *testing.T) {
	testCases := []struct {
		name  string
		event github.WorkflowJobEvent
	}{
		{
			"validEvent",
			github.WorkflowJobEvent{
				WorkflowJob: &github.WorkflowJob{
					WorkflowName: github.Ptr("test"),
					Steps: []*github.TaskStep{
						{
							Name:        github.Ptr("test1"),
							StartedAt:   &github.Timestamp{Time: time.Now()},
							CompletedAt: &github.Timestamp{Time: time.Now()},
						},
						{
							Name:        github.Ptr("test2"),
							StartedAt:   &github.Timestamp{Time: time.Now()},
							CompletedAt: &github.Timestamp{Time: time.Now()},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaultConfig := createDefaultConfig().(*Config)
			defaultConfig.WebHook.Endpoint = "localhost:0"
			consumer := &consumertest.TracesSink{}
			receiver, err := newTracesReceiver(receivertest.NewNopSettings(), defaultConfig, consumer)
			require.NoError(t, err, "failed to create receiver")

			r := receiver
			require.NoError(t, r.Start(context.Background(), componenttest.NewNopHost()), "failed to start receiver")
			defer func() {
				require.NoError(t, r.Shutdown(context.Background()), "failed to shutdown revceiver")
			}()

			body, err := json.Marshal(tc.event)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "http://localhost/event", bytes.NewReader(body))
			request.Header.Add("Content-Type", "application/json")
			// https://docs.github.com/en/webhooks/webhook-events-and-payloads
			request.Header.Add("X-Github-Event", "workflow_job")
			r.handleWebhook(w, request)

			response := w.Result()
			body, err = io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode, string(body))

			traces := consumer.AllTraces()

			require.Equal(t, len(traces), 1)
		})
	}
}

func tracesFromEvent(event *github.WorkflowJobEvent) ptrace.Traces {
	trace := ptrace.NewTraces()

	return trace
}
