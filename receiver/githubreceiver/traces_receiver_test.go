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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
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
				Action: github.Ptr(COMPLETED),
				WorkflowJob: &github.WorkflowJob{
					WorkflowName: github.Ptr("test"),
					StartedAt:    &github.Timestamp{Time: time.Now().Add(time.Second)},
					CompletedAt:  &github.Timestamp{Time: time.Now().Add(time.Second * 2)},
					Steps: []*github.TaskStep{
						{
							Name:        github.Ptr("test1"),
							StartedAt:   &github.Timestamp{Time: time.Now().Add(time.Second * 3)},
							CompletedAt: &github.Timestamp{Time: time.Now().Add(time.Second * 4)},
						},
						{
							Name:        github.Ptr("test2"),
							StartedAt:   &github.Timestamp{Time: time.Now().Add(time.Second * 5)},
							CompletedAt: &github.Timestamp{Time: time.Now().Add(time.Second * 6)},
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

			traces := clearTraces(consumer.AllTraces())

			require.Equal(t, tracesFromEvent(&tc.event), traces)
		})
	}
}

func tracesFromEvent(event *github.WorkflowJobEvent) []ptrace.Traces {
	trace := ptrace.NewTraces()

	rs := trace.ResourceSpans().AppendEmpty()
	attributes := rs.Resource().Attributes()
	attributes.PutStr(semconv.AttributeServiceName, defaultServiceName)
	attributes.PutStr("github.workflow.name", *event.WorkflowJob.WorkflowName)
	rs.SetSchemaUrl(semconv.SchemaURL)

	ss := rs.ScopeSpans().AppendEmpty()

	createSpan(event.WorkflowJob).CopyTo(ss.Spans().AppendEmpty())
	for _, step := range event.WorkflowJob.Steps {
		span := createSpan(step)

		span.CopyTo(ss.Spans().AppendEmpty())
	}

	return []ptrace.Traces{trace}
}

func clearTraces(traces []ptrace.Traces) []ptrace.Traces {
	newTraces := make([]ptrace.Traces, len(traces))

	for i, trace := range traces {
		newTrace := ptrace.NewTraces()
		trace.CopyTo(newTrace)

		for i := 0; i < trace.ResourceSpans().Len(); i++ {
			rs := newTrace.ResourceSpans().At(i)

			for i := 0; i < rs.ScopeSpans().Len(); i++ {
				ss := rs.ScopeSpans().At(i)

				for i := 0; i < ss.Spans().Len(); i++ {
					span := ss.Spans().At(i)
					span.SetTraceID(pcommon.NewTraceIDEmpty())
					span.SetSpanID(pcommon.NewSpanIDEmpty())
					span.SetParentSpanID(pcommon.NewSpanIDEmpty())
				}
			}
		}

		newTraces[i] = newTrace
	}

	return newTraces
}
