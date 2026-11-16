// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

// signalBackend is an OTLP/HTTP backend that records the request paths and the services it received.
type signalBackend struct {
	*httptest.Server
	mu       sync.Mutex
	paths    map[string]int
	services map[string]int
}

// newSignalBackend decodes the body with decode, which returns the service names found in it.
func newSignalBackend(t *testing.T, decode func([]byte) ([]string, error)) *signalBackend {
	t.Helper()
	b := &signalBackend{paths: map[string]int{}, services: map[string]int{}}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		svcs, err := decode(body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b.mu.Lock()
		b.paths[r.URL.Path]++
		for _, s := range svcs {
			b.services[s]++
		}
		b.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(b.Close)
	return b
}

func decodeTraces(body []byte) ([]string, error) {
	req := ptraceotlp.NewExportRequest()
	if err := req.UnmarshalProto(body); err != nil {
		return nil, err
	}
	var out []string
	rss := req.Traces().ResourceSpans()
	for i := range rss.Len() {
		svc, _ := rss.At(i).Resource().Attributes().Get("service.name")
		out = append(out, svc.Str())
	}
	return out, nil
}

func decodeLogs(body []byte) ([]string, error) {
	req := plogotlp.NewExportRequest()
	if err := req.UnmarshalProto(body); err != nil {
		return nil, err
	}
	var out []string
	rls := req.Logs().ResourceLogs()
	for i := range rls.Len() {
		svc, _ := rls.At(i).Resource().Attributes().Get("service.name")
		out = append(out, svc.Str())
	}
	return out, nil
}

// checkSpread asserts that every service reached exactly one backend, every send arrived, and the data was
// spread over more than one backend. It also asserts the path that the OTLP/HTTP exporter used.
func checkSpread(t *testing.T, backends []*signalBackend, services []string, sends int, path string) {
	t.Helper()
	owner := map[string]int{}
	used := 0
	for i, b := range backends {
		b.mu.Lock()
		if len(b.services) > 0 {
			used++
		}
		for p := range b.paths {
			assert.Equal(t, path, p)
		}
		for svc, n := range b.services {
			_, dup := owner[svc]
			assert.False(t, dup, "%s was sent to more than one backend", svc)
			owner[svc] = i
			assert.Equal(t, sends, n, "every send of %s reaches the same backend", svc)
		}
		b.mu.Unlock()
	}
	assert.Len(t, owner, len(services), "no data lost")
	assert.Greater(t, used, 1, "data is spread over backends")
}

func serviceNames(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("svc-%d", i)
	}
	return out
}

func hostsOf(backends []*signalBackend) []string {
	hosts := make([]string, len(backends))
	for i, b := range backends {
		hosts[i] = b.Listener.Addr().String()
	}
	return hosts
}

func TestOTLPHTTPRoutesTracesToOneBackend(t *testing.T) {
	backends := []*signalBackend{
		newSignalBackend(t, decodeTraces), newSignalBackend(t, decodeTraces), newSignalBackend(t, decodeTraces),
	}
	cfg := syncHTTPConfig(hostsOf(backends), func(h *otlphttpexporter.Config) {
		h.TracesEndpoint = "http://placeholder/custom/v1/traces"
	})
	cfg.RoutingKey = svcRoutingStr

	ctx := t.Context()
	te, err := createTracesExporter(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, te.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, te.Shutdown(t.Context())) })

	services := serviceNames(30)
	const sends = 3
	for range sends {
		td := ptrace.NewTraces()
		for _, svc := range services {
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("service.name", svc)
			span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			span.SetTraceID(pcommon.TraceID{1})
			span.SetSpanID(pcommon.SpanID{1})
		}
		require.NoError(t, te.ConsumeTraces(ctx, td))
	}
	checkSpread(t, backends, services, sends, "/custom/v1/traces")
}

func TestOTLPHTTPRoutesLogsToOneBackend(t *testing.T) {
	backends := []*signalBackend{
		newSignalBackend(t, decodeLogs), newSignalBackend(t, decodeLogs), newSignalBackend(t, decodeLogs),
	}
	// Only endpoint is set, so the exporter appends /v1/logs as the OTLP/HTTP exporter does.
	cfg := syncHTTPConfig(hostsOf(backends), func(h *otlphttpexporter.Config) {
		h.ClientConfig.Endpoint = "http://placeholder"
	})
	cfg.RoutingKey = svcRoutingStr

	ctx := t.Context()
	le, err := createLogsExporter(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, le.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, le.Shutdown(t.Context())) })

	services := serviceNames(30)
	const sends = 3
	for range sends {
		ld := plog.NewLogs()
		for _, svc := range services {
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", svc)
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("line")
		}
		require.NoError(t, le.ConsumeLogs(ctx, ld))
	}
	checkSpread(t, backends, services, sends, "/v1/logs")
}
