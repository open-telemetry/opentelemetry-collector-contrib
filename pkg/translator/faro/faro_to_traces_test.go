// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package faro // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro"

import (
	"context"
	"encoding/hex"
	"testing"

	faroTypes "github.com/grafana/faro/pkg/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
)

func TestTranslateToTraces(t *testing.T) {
	testcases := []struct {
		name           string
		faroPayload    faroTypes.Payload
		expectedTraces *ptrace.Traces
		wantErr        assert.ErrorAssertionFunc
	}{
		{
			name:           "Standard payload",
			faroPayload:    *PayloadFromFile(t, "payload.json"),
			expectedTraces: generateTraces(t),
			wantErr:        assert.NoError,
		},
		{
			name:           "Empty payload",
			faroPayload:    faroTypes.Payload{},
			expectedTraces: nil,
			wantErr:        assert.NoError,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			traces, err := TranslateToTraces(context.TODO(), tt.faroPayload)
			if !tt.wantErr(t, err) {
				return
			}
			if tt.expectedTraces == nil && assert.Nil(t, traces) {
				return
			}
			assert.NoError(t, ptracetest.CompareTraces(*tt.expectedTraces, *traces))
		})
	}
}

func generateTraces(t *testing.T) *ptrace.Traces {
	t.Helper()
	traces := ptrace.NewTraces()
	resourceTraces := traces.ResourceSpans().AppendEmpty()
	_ = resourceTraces.Resource().Attributes().FromRaw(map[string]any{
		string(semconv.ServiceNameKey):           "testapp",
		string(semconv.ServiceVersionKey):        "abcdefg",
		string(semconv.ServiceNamespaceKey):      "testnamespace",
		string(semconv.DeploymentEnvironmentKey): "production",
		"telemetry.sdk.language":                 "webjs",
		"telemetry.sdk.name":                     "opentelemetry",
		"telemetry.sdk.version":                  "1.21.0",
	})

	scopeSpans := resourceTraces.ScopeSpans().AppendEmpty()
	scope := scopeSpans.Scope()
	scope.SetName("@opentelemetry/instrumentation-fetch")
	scope.SetVersion("0.48.0")

	span := scopeSpans.Spans().AppendEmpty()
	span.SetName("HTTP GET")

	var spanID [8]byte
	_, err := hex.Decode(spanID[:], []byte("f71b2cc42962650f"))
	require.NoError(t, err)
	span.SetSpanID(spanID)

	var traceID [16]byte
	_, err = hex.Decode(traceID[:], []byte("bac44bf5d6d040fc5fdf9bc22442c6f2"))
	require.NoError(t, err)
	span.SetTraceID(traceID)

	span.SetKind(3)
	span.SetStartTimestamp(pcommon.Timestamp(1718700770771000000))
	span.SetEndTimestamp(pcommon.Timestamp(1718700770800000000))
	err = span.Attributes().FromRaw(map[string]any{
		"component":                    "fetch",
		"http.method":                  "GET",
		"http.url":                     "http://localhost:5173/",
		"session_id":                   "cD3am6QTPa",
		"http.status_code":             200,
		"http.status_text":             "OK",
		"http.host":                    "localhost:5173",
		"http.scheme":                  "http",
		"http.user_agent":              "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:128.0) Gecko/20100101 Firefox/128.0",
		"http.response_content_length": 2819,
	})
	require.NoError(t, err)
	events := []struct {
		Name      string
		Timestamp uint64
	}{
		{
			Name:      "fetchStart",
			Timestamp: 1718700770771000000,
		},
		{
			Name:      "domainLookupStart",
			Timestamp: 1718700770775000000,
		},
		{
			Name:      "domainLookupEnd",
			Timestamp: 1718700770775000000,
		},
		{
			Name:      "connectStart",
			Timestamp: 1718700770775000000,
		},
		{
			Name:      "connectEnd",
			Timestamp: 1718700770775000000,
		},
		{
			Name:      "requestStart",
			Timestamp: 1718700770775000000,
		},
		{
			Name:      "responseStart",
			Timestamp: 1718700770797000000,
		},
		{
			Name:      "responseEnd",
			Timestamp: 1718700770797000000,
		},
	}

	evts := span.Events()
	for _, event := range events {
		evt := evts.AppendEmpty()
		evt.SetName(event.Name)
		evt.SetTimestamp(pcommon.Timestamp(event.Timestamp))
	}
	evts.CopyTo(span.Events())
	return &traces
}
