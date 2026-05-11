// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestDrainTogglesReadinessAndHealth(t *testing.T) {
	s := &server{started: time.Now()}

	assertServing(t, s, true)

	recorder := httptest.NewRecorder()
	s.handleDrain(recorder, httptest.NewRequest(http.MethodPost, "/drain", http.NoBody))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("drain status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	assertServing(t, s, false)

	recorder = httptest.NewRecorder()
	s.handleUndrain(recorder, httptest.NewRequest(http.MethodPost, "/undrain", http.NoBody))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("undrain status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	assertServing(t, s, true)
}

func assertServing(t *testing.T, s *server, want bool) {
	t.Helper()

	if got := !s.isUnready(); got != want {
		t.Fatalf("ready = %v, want %v", got, want)
	}

	resp, err := s.Check(t.Context(), &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	wantStatus := healthpb.HealthCheckResponse_NOT_SERVING
	if want {
		wantStatus = healthpb.HealthCheckResponse_SERVING
	}
	if resp.GetStatus() != wantStatus {
		t.Fatalf("health status = %s, want %s", resp.GetStatus(), wantStatus)
	}
}
