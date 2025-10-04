// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNopNotifier(t *testing.T) {
	nop := NewNop()
	assert.NotNil(t, nop)

	err := nop.Send(context.Background(), []Event{
		{Rule: "test", State: "firing"},
	})
	assert.NoError(t, err)

	// Should work with empty events too
	err = nop.Send(context.Background(), []Event{})
	assert.NoError(t, err)
}

func TestAlertmanagerNotifier(t *testing.T) {
	t.Run("successful_send", func(t *testing.T) {
		var receivedPayload []map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v2/alerts", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			err = json.Unmarshal(body, &receivedPayload)
			require.NoError(t, err)

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		am := NewAlertmanager(server.URL, 5*time.Second)

		events := []Event{
			{
				Rule:     "high_latency",
				State:    "firing",
				Severity: "critical",
				Labels: map[string]string{
					"service": "api",
					"env":     "prod",
				},
				Value:  0.95,
				Window: "5m",
				For:    "1m",
			},
			{
				Rule:     "error_rate",
				State:    "resolved",
				Severity: "warning",
				Labels: map[string]string{
					"service": "db",
				},
				Value:  0.02,
				Window: "10m",
				For:    "2m",
			},
		}

		err := am.Send(context.Background(), events)
		require.NoError(t, err)

		// Verify payload
		assert.Len(t, receivedPayload, 2)

		// Check first alert
		alert1 := receivedPayload[0]
		labels1 := alert1["labels"].(map[string]interface{})
		assert.Equal(t, "high_latency", labels1["alertname"])
		assert.Equal(t, "critical", labels1["severity"])
		assert.Equal(t, "firing", labels1["state"])
		assert.Equal(t, "api", labels1["service"])
		assert.Equal(t, "prod", labels1["env"])

		annotations1 := alert1["annotations"].(map[string]interface{})
		assert.Equal(t, "0.95", annotations1["value"])
		assert.Equal(t, "5m", annotations1["window"])
		assert.Equal(t, "1m", annotations1["for"])

		// Check second alert
		alert2 := receivedPayload[1]
		labels2 := alert2["labels"].(map[string]interface{})
		assert.Equal(t, "error_rate", labels2["alertname"])
		assert.Equal(t, "resolved", labels2["state"])
	})

	t.Run("server_error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		am := NewAlertmanager(server.URL, 1*time.Second)

		events := []Event{
			{Rule: "test", State: "firing"},
		}

		err := am.Send(context.Background(), events)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alertmanager responded")
	})

	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		am := NewAlertmanager(server.URL, 100*time.Millisecond)

		events := []Event{
			{Rule: "test", State: "firing"},
		}

		err := am.Send(context.Background(), events)
		assert.Error(t, err)
	})

	t.Run("nil_alertmanager", func(t *testing.T) {
		var am *Alertmanager
		err := am.Send(context.Background(), []Event{{Rule: "test"}})
		assert.NoError(t, err)
	})

	t.Run("empty_base_url", func(t *testing.T) {
		am := &Alertmanager{
			Client:  &http.Client{},
			BaseURL: "",
		}
		err := am.Send(context.Background(), []Event{{Rule: "test"}})
		assert.NoError(t, err)
	})

	t.Run("empty_events", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Should not be called with empty events")
		}))
		defer server.Close()

		am := NewAlertmanager(server.URL, 1*time.Second)
		err := am.Send(context.Background(), []Event{})
		assert.NoError(t, err)
	})

	t.Run("starts_at_ends_at", func(t *testing.T) {
		var receivedPayload []map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedPayload)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		am := NewAlertmanager(server.URL, 1*time.Second)

		beforeTime := time.Now()

		events := []Event{
			{Rule: "test", State: "firing"},
			{Rule: "test2", State: "resolved"},
		}

		err := am.Send(context.Background(), events)
		require.NoError(t, err)

		afterTime := time.Now()

		// Check startsAt for both alerts
		for _, alert := range receivedPayload {
			startsAtStr, ok := alert["startsAt"].(string)
			assert.True(t, ok)

			startsAt, err := time.Parse(time.RFC3339, startsAtStr)
			require.NoError(t, err)

			assert.True(t, startsAt.After(beforeTime) || startsAt.Equal(beforeTime))
			assert.True(t, startsAt.Before(afterTime) || startsAt.Equal(afterTime))
		}

		// Check endsAt for resolved alert
		alert2 := receivedPayload[1]
		labels2 := alert2["labels"].(map[string]interface{})
		if labels2["alertname"] == "test2" {
			endsAtStr, ok := alert2["endsAt"].(string)
			assert.True(t, ok)

			endsAt, err := time.Parse(time.RFC3339, endsAtStr)
			require.NoError(t, err)

			assert.True(t, endsAt.After(beforeTime) || endsAt.Equal(beforeTime))
			assert.True(t, endsAt.Before(afterTime) || endsAt.Equal(afterTime))
		}

		// Firing alert should not have endsAt set to current time
		alert1 := receivedPayload[0]
		labels1 := alert1["labels"].(map[string]interface{})
		if labels1["alertname"] == "test" && labels1["state"] == "firing" {
			endsAtStr, ok := alert1["endsAt"].(string)
			if ok && endsAtStr != "" {
				endsAt, err := time.Parse(time.RFC3339, endsAtStr)
				if err == nil {
					// endsAt should be zero time for firing alerts
					assert.True(t, endsAt.IsZero() || endsAt.After(afterTime))
				}
			}
		}
	})

	t.Run("generator_url", func(t *testing.T) {
		var receivedPayload []map[string]any

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedPayload)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		am := NewAlertmanager(server.URL, 1*time.Second)

		events := []Event{{Rule: "test", State: "firing"}}
		err := am.Send(context.Background(), events)
		require.NoError(t, err)

		alert := receivedPayload[0]
		assert.Equal(t, "otel/alertsgen", alert["generatorURL"])
	})
}
