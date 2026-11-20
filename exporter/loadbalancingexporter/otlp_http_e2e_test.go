// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

func metricsFor(services ...string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for _, svc := range services {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("service.name", svc)
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("metric")
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	}
	return md
}

// metricsBackend is an OTLP/HTTP backend that records which services it received.
type metricsBackend struct {
	*httptest.Server
	mu       sync.Mutex
	services map[string]int
}

func newMetricsBackend(t *testing.T) *metricsBackend {
	t.Helper()
	b := &metricsBackend{services: map[string]int{}}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		req := pmetricotlp.NewExportRequest()
		if err := req.UnmarshalProto(body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		b.mu.Lock()
		rms := req.Metrics().ResourceMetrics()
		for i := range rms.Len() {
			svc, _ := rms.At(i).Resource().Attributes().Get("service.name")
			b.services[svc.Str()]++
		}
		b.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(b.Close)
	return b
}

func (b *metricsBackend) seen() map[string]int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return maps.Clone(b.services)
}

func newHTTPMetricsExporter(t *testing.T, cfg *Config) func(pmetric.Metrics) error {
	t.Helper()
	ctx := t.Context()
	me, err := createMetricsExporter(ctx, exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, me.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, me.Shutdown(context.WithoutCancel(ctx))) })
	return func(md pmetric.Metrics) error { return me.ConsumeMetrics(ctx, md) }
}

func syncHTTPConfig(hostnames []string, mutate func(*otlphttpexporter.Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings = configoptional.None[exporterhelper.QueueBatchConfig]()
	cfg.Resolver.Static = configoptional.Some(StaticResolver{Hostnames: hostnames})
	h := httpDefaults()
	h.ClientConfig.Compression = ""
	h.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
	h.RetryConfig.Enabled = false
	mutate(h)
	cfg.Protocol.OTLPHTTP = configoptional.Some(*h)
	return cfg
}

func TestOTLPHTTPRoutesSeriesToOneBackend(t *testing.T) {
	backends := []*metricsBackend{newMetricsBackend(t), newMetricsBackend(t), newMetricsBackend(t)}
	var hosts []string
	for _, b := range backends {
		hosts = append(hosts, b.Listener.Addr().String())
	}
	cfg := syncHTTPConfig(hosts, func(h *otlphttpexporter.Config) {
		h.MetricsEndpoint = "http://placeholder/opentelemetry/v1/metrics"
	})
	cfg.RoutingKey = attrRoutingStr
	cfg.RoutingAttributes = []string{"service.name"}
	send := newHTTPMetricsExporter(t, cfg)

	var services []string
	for i := range 30 {
		services = append(services, fmt.Sprintf("svc-%d", i))
	}
	for range 3 {
		require.NoError(t, send(metricsFor(services...)))
	}

	owner := map[string]int{}
	used := 0
	for i, b := range backends {
		seen := b.seen()
		if len(seen) > 0 {
			used++
		}
		for svc, n := range seen {
			_, dup := owner[svc]
			assert.False(t, dup, "%s was sent to more than one backend", svc)
			owner[svc] = i
			assert.Equal(t, 3, n, "every send of %s reaches the same backend", svc)
		}
	}
	assert.Len(t, owner, len(services), "no data lost")
	assert.Greater(t, used, 1, "data is spread over backends")
}

func TestOTLPHTTPRetriesTransientErrors(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cfg := syncHTTPConfig([]string{srv.Listener.Addr().String()}, func(h *otlphttpexporter.Config) {
		h.ClientConfig.Endpoint = "http://placeholder"
		h.RetryConfig.Enabled = true
		h.RetryConfig.InitialInterval = 5 * time.Millisecond
		h.RetryConfig.MaxInterval = 10 * time.Millisecond
	})
	require.NoError(t, newHTTPMetricsExporter(t, cfg)(metricsFor("svc")))
	assert.EqualValues(t, 3, calls.Load())
}

func TestOTLPHTTPPermanentErrorIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	cfg := syncHTTPConfig([]string{srv.Listener.Addr().String()}, func(h *otlphttpexporter.Config) {
		h.ClientConfig.Endpoint = "http://placeholder"
		h.RetryConfig.Enabled = true
		h.RetryConfig.InitialInterval = 5 * time.Millisecond
	})
	require.Error(t, newHTTPMetricsExporter(t, cfg)(metricsFor("svc")))
	assert.EqualValues(t, 1, calls.Load())
}

func TestOTLPHTTPUnreachableBackendReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.Listener.Addr().String()
	srv.Close()

	cfg := syncHTTPConfig([]string{addr}, func(h *otlphttpexporter.Config) {
		h.ClientConfig.Endpoint = "http://placeholder"
		h.ClientConfig.Timeout = time.Second
	})
	require.Error(t, newHTTPMetricsExporter(t, cfg)(metricsFor("svc")))
}

func TestOTLPHTTPClientTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { <-release }))
	t.Cleanup(func() { close(release); srv.Close() })

	cfg := syncHTTPConfig([]string{srv.Listener.Addr().String()}, func(h *otlphttpexporter.Config) {
		h.ClientConfig.Endpoint = "http://placeholder"
		h.ClientConfig.Timeout = 50 * time.Millisecond
	})
	start := time.Now()
	require.Error(t, newHTTPMetricsExporter(t, cfg)(metricsFor("svc")))
	assert.Less(t, time.Since(start), 5*time.Second)
}

// TestOTLPHTTPBackendChanges checks that resolved backends without a port get 4318 and that
// exporters are added and removed when the resolver changes.
func TestOTLPHTTPBackendChanges(t *testing.T) {
	cfg := syncHTTPConfig([]string{"a"}, func(h *otlphttpexporter.Config) { h.ClientConfig.Endpoint = "http://placeholder" })
	var mu sync.Mutex
	var created []string
	factory := func(_ context.Context, endpoint string) (component.Component, error) {
		mu.Lock()
		created = append(created, endpoint)
		mu.Unlock()
		return mockComponent{}, nil
	}
	lb, err := newLoadBalancer(exportertest.NewNopSettings(metadata.Type).Logger, cfg, factory, nil)
	require.NoError(t, err)
	lb.host = componenttest.NewNopHost()

	lb.onBackendChanges([]string{"a", "b:9000"})
	assert.ElementsMatch(t, []string{"a:4318", "b:9000"}, created)
	assert.Equal(t, 2, lb.NumBackends())

	lb.onBackendChanges([]string{"b:9000", "c"})
	assert.ElementsMatch(t, []string{"a:4318", "b:9000", "c:4318"}, created)
	assert.Equal(t, 2, lb.NumBackends())
	lb.exportersShutdownWG.Wait()
}
