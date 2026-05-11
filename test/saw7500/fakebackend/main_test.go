// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	logspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
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
	gotServing := resp.GetStatus() == healthpb.HealthCheckResponse_SERVING
	if gotServing != want {
		t.Fatalf("health serving = %v, want %v", gotServing, want)
	}
}

func TestLoadgenPayloadRepeatedUsesExactByteSize(t *testing.T) {
	payload, err := loadgenPayload(loadgenConfig{
		payloadProfile:    "repeated",
		payloadSizeBytes:  256 * 1024,
		body:              "fallback",
		randomPayloadSeed: 1,
	})
	if err != nil {
		t.Fatalf("loadgenPayload failed: %v", err)
	}

	if len(payload) != 256*1024 {
		t.Fatalf("payload size = %d, want %d", len(payload), 256*1024)
	}
	if strings.Trim(payload, "A") != "" {
		t.Fatalf("repeated payload contains bytes other than A")
	}
}

func TestLoadgenPayloadRandomUsesExactByteSizeAndIsNotConstant(t *testing.T) {
	payload, err := loadgenPayload(loadgenConfig{
		payloadProfile:    "random",
		payloadSizeBytes:  1024,
		body:              "fallback",
		randomPayloadSeed: 1,
	})
	if err != nil {
		t.Fatalf("loadgenPayload failed: %v", err)
	}

	if len(payload) != 1024 {
		t.Fatalf("payload size = %d, want %d", len(payload), 1024)
	}
	if strings.Trim(payload, string(payload[0])) == "" {
		t.Fatalf("random payload is constant")
	}
}

func TestBuildLoadgenRequestSetsServiceTenantAndBatch(t *testing.T) {
	req := buildLoadgenRequest("payload", 3, "bigid-local-saw-7500", "bigid")

	if got := countLogRecords(req); got != 3 {
		t.Fatalf("record count = %d, want 3", got)
	}
	resourceAttrs := req.GetResourceLogs()[0].GetResource().GetAttributes()
	if got := resourceAttrs[0].GetKey(); got != "service.name" {
		t.Fatalf("resource attr key = %q, want service.name", got)
	}
	if got := resourceAttrs[0].GetValue().GetStringValue(); got != "bigid-local-saw-7500" {
		t.Fatalf("service.name = %q, want bigid-local-saw-7500", got)
	}
	recordAttrs := req.GetResourceLogs()[0].GetScopeLogs()[0].GetLogRecords()[0].GetAttributes()
	if got := recordAttrs[0].GetKey(); got != "tenant" {
		t.Fatalf("record attr key = %q, want tenant", got)
	}
	if got := recordAttrs[0].GetValue().GetStringValue(); got != "bigid" {
		t.Fatalf("tenant = %q, want bigid", got)
	}
}

func TestShouldLogLoadgenFailureRateLimitsRepeats(t *testing.T) {
	base := time.Unix(100, 0)

	if !shouldLogLoadgenFailure(1, time.Time{}, base) {
		t.Fatalf("first failure should log")
	}
	if shouldLogLoadgenFailure(2, base, base.Add(time.Second)) {
		t.Fatalf("second failure one second later should not log")
	}
	if !shouldLogLoadgenFailure(100, base, base.Add(time.Second)) {
		t.Fatalf("hundredth failure should log")
	}
	if !shouldLogLoadgenFailure(2, base, base.Add(30*time.Second)) {
		t.Fatalf("failure after 30 seconds should log")
	}
}

func TestRunLoadgenWorkerDoesNotCountFailedExportsAsGenerated(t *testing.T) {
	generated, err := runLoadgenWorker(
		t.Context(),
		fakeLogsClient{err: errors.New("backend unavailable")},
		loadgenConfig{
			duration:      25 * time.Millisecond,
			rate:          1000,
			workers:       1,
			batchSize:     1,
			allowFailures: true,
			serviceName:   "test",
			tenant:        "test",
		},
		"payload",
		0,
	)
	if err != nil {
		t.Fatalf("runLoadgenWorker failed: %v", err)
	}
	if generated != 0 {
		t.Fatalf("generated = %d, want 0 for failed exports", generated)
	}
}

type fakeLogsClient struct {
	err error
}

func (c fakeLogsClient) Export(
	context.Context,
	*logspb.ExportLogsServiceRequest,
	...grpc.CallOption,
) (*logspb.ExportLogsServiceResponse, error) {
	return &logspb.ExportLogsServiceResponse{}, c.err
}
