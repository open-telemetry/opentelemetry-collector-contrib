// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// logDelivery records which endpoint received which resource.
type logDelivery struct {
	mu sync.Mutex
	// records counts log records per endpoint.
	records map[string]int
	// endpointByHost maps the host.name resource attribute to the endpoints that received it.
	endpointByHost map[string]map[string]struct{}
	// endpointBySvc maps the service.name resource attribute to the endpoints that received it.
	endpointBySvc map[string]map[string]struct{}
}

func newLogDelivery() *logDelivery {
	return &logDelivery{
		records:        map[string]int{},
		endpointByHost: map[string]map[string]struct{}{},
		endpointBySvc:  map[string]map[string]struct{}{},
	}
}

func (d *logDelivery) record(endpoint string, ld plog.Logs) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.records[endpoint] += ld.LogRecordCount()
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		attrs := ld.ResourceLogs().At(i).Resource().Attributes()
		if v, ok := attrs.Get("host.name"); ok {
			if d.endpointByHost[v.Str()] == nil {
				d.endpointByHost[v.Str()] = map[string]struct{}{}
			}
			d.endpointByHost[v.Str()][endpoint] = struct{}{}
		}
		if v, ok := attrs.Get("service.name"); ok {
			if d.endpointBySvc[v.Str()] == nil {
				d.endpointBySvc[v.Str()] = map[string]struct{}{}
			}
			d.endpointBySvc[v.Str()][endpoint] = struct{}{}
		}
	}
}

// newServiceRoutingLogsExporter builds the real logs exporter and the real load balancer
// with a hash ring over the endpoints. Only the per-endpoint sub-exporter is a mock.
func newServiceRoutingLogsExporter(t *testing.T, routingKey string, endpoints []string, d *logDelivery) *logExporterImp {
	t.Helper()
	ts, tb := getTelemetryAssets(t)

	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			d.record(endpoint, ld)
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: routingKey,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), endpoints)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })
	return p
}

func unnamedResourceLog(host string) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("host.name", host)
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log from " + host)
	return ld
}

// TestE2EMissingServiceNameSpreadsAcrossBackends is an end-to-end check through the real
// exporter and the real hash ring. It proves that resources without service.name
//  1. are not all sent to one backend,
//  2. always reach the same backend (per-resource affinity),
//  3. lose no log records and are delivered exactly once.
func TestE2EMissingServiceNameSpreadsAcrossBackends(t *testing.T) {
	const (
		resources = 90
		rounds    = 3
	)
	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	d := newLogDelivery()
	p := newServiceRoutingLogsExporter(t, svcRoutingStr, endpoints, d)

	for round := range rounds {
		for i := range resources {
			require.NoError(t, p.ConsumeLogs(t.Context(), unnamedResourceLog(fmt.Sprintf("host-%d", i))), "round %d", round)
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	total := 0
	for _, n := range d.records {
		total += n
	}
	assert.Equal(t, resources*rounds, total, "every record is delivered exactly once")

	t.Logf("records per endpoint (%d resources x %d rounds): %v", resources, rounds, d.records)
	assert.Len(t, d.records, len(endpoints), "every endpoint receives data")
	for ep, n := range d.records {
		share := float64(n) / float64(total)
		assert.Less(t, share, 0.6, "endpoint %s holds %.0f%% of the data", ep, share*100)
	}

	for host, eps := range d.endpointByHost {
		assert.Len(t, eps, 1, "resource %s must stay on one backend", host)
	}
	assert.Len(t, d.endpointByHost, resources)
}

// TestE2EMissingServiceNameSameAsResourceRouting shows that unnamed resources follow the
// same backend as routing_key "resource", so both routing keys agree on placement.
func TestE2EMissingServiceNameSameAsResourceRouting(t *testing.T) {
	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	svcDelivery, resDelivery := newLogDelivery(), newLogDelivery()
	svcExporter := newServiceRoutingLogsExporter(t, svcRoutingStr, endpoints, svcDelivery)
	resExporter := newServiceRoutingLogsExporter(t, resourceRoutingStr, endpoints, resDelivery)

	for i := range 60 {
		ld := unnamedResourceLog(fmt.Sprintf("host-%d", i))
		require.NoError(t, svcExporter.ConsumeLogs(t.Context(), ld))
		require.NoError(t, resExporter.ConsumeLogs(t.Context(), ld))
	}

	svcDelivery.mu.Lock()
	defer svcDelivery.mu.Unlock()
	resDelivery.mu.Lock()
	defer resDelivery.mu.Unlock()
	assert.Equal(t, resDelivery.endpointByHost, svcDelivery.endpointByHost)
}

// TestE2ENamedServicesKeepServiceAffinity proves that the change does not affect resources
// that have a service.name: all resources of one service still go to one backend, even when
// their other resource attributes differ.
func TestE2ENamedServicesKeepServiceAffinity(t *testing.T) {
	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	d := newLogDelivery()
	p := newServiceRoutingLogsExporter(t, svcRoutingStr, endpoints, d)

	services := []string{"payment", "auth", "order", "inventory", "shipping", "notify"}
	for _, svc := range services {
		for i := range 20 {
			ld := plog.NewLogs()
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", svc)
			rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("%s-host-%d", svc, i))
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
			require.NoError(t, p.ConsumeLogs(t.Context(), ld))
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	for _, svc := range services {
		assert.Len(t, d.endpointBySvc[svc], 1, "service %s must stay on one backend", svc)
	}
}

// TestE2EMixedBatchDeliversEveryRecordOnce sends named, unnamed and empty-name resources in
// one batch and checks that no record is lost or duplicated.
func TestE2EMixedBatchDeliversEveryRecordOnce(t *testing.T) {
	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	d := newLogDelivery()
	p := newServiceRoutingLogsExporter(t, svcRoutingStr, endpoints, d)

	ld := plog.NewLogs()
	want := 0
	for i := range 30 {
		rl := ld.ResourceLogs().AppendEmpty()
		switch i % 3 {
		case 0:
			rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		case 1:
			rl.Resource().Attributes().PutStr("service.name", "")
		}
		rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		for range 2 {
			lrs.AppendEmpty().Body().SetStr("log")
			want++
		}
	}
	require.NoError(t, p.ConsumeLogs(t.Context(), ld))

	d.mu.Lock()
	defer d.mu.Unlock()
	got := 0
	for _, n := range d.records {
		got += n
	}
	assert.Equal(t, want, got)
}

func TestSplitLogsByServiceNameFallback(t *testing.T) {
	t.Run("unnamed resource is keyed by resource identity", func(t *testing.T) {
		ld := unnamedResourceLog("host-1")
		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 1)
		want := identity.OfResource(ld.ResourceLogs().At(0).Resource()).String()
		require.Contains(t, batches, want)
		assert.NotEmpty(t, want)
	})

	t.Run("empty service name counts as missing", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 1)
		assert.NotContains(t, batches, "")
	})

	t.Run("different unnamed resources get different keys", func(t *testing.T) {
		ld := plog.NewLogs()
		for i := range 5 {
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
		}
		assert.Len(t, splitLogsByServiceName(ld), 5)
	})

	t.Run("identical unnamed resources are merged into one batch", func(t *testing.T) {
		ld := plog.NewLogs()
		for range 3 {
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("host.name", "host-1")
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log")
		}
		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 1)
		for _, b := range batches {
			assert.Equal(t, 3, b.LogRecordCount())
		}
	})

	t.Run("key is deterministic", func(t *testing.T) {
		a := splitLogsByServiceName(unnamedResourceLog("host-1"))
		b := splitLogsByServiceName(unnamedResourceLog("host-1"))
		for k := range a {
			assert.Contains(t, b, k)
		}
	})

	t.Run("named resources keep the service name as key", func(t *testing.T) {
		batches := splitLogsByServiceName(simpleLogWithServiceName("svc-1"))
		require.Len(t, batches, 1)
		assert.Contains(t, batches, "svc-1")
	})
}

// FuzzSplitLogsByServiceName checks the invariants of the split for arbitrary resource
// layouts. Each input byte describes one resource.
func FuzzSplitLogsByServiceName(f *testing.F) {
	seeds := [][]byte{
		{},
		{0},
		{1},
		{0, 0, 0},
		{1, 1, 1},
		{0, 1, 0, 1},
		{0xff, 0x00, 0xaa, 0x55},
		{3, 7, 11, 15, 19, 23},
		{0x10, 0x20, 0x30, 0x40, 0x50},
		{2, 2, 2, 2, 2, 2, 2, 2},
		{0xfe, 0xfd, 0xfc, 0xfb},
		{1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0},
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		ld := plog.NewLogs()
		wantRecords := 0
		for _, b := range data {
			rl := ld.ResourceLogs().AppendEmpty()
			attrs := rl.Resource().Attributes()
			switch b % 3 {
			case 0: // named
				attrs.PutStr("service.name", fmt.Sprintf("svc-%d", b>>2))
			case 1: // empty service name
				attrs.PutStr("service.name", "")
			default: // missing
			}
			attrs.PutStr("host.name", fmt.Sprintf("host-%d", b>>4))
			lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
			n := int(b>>6) + 1
			for range n {
				lrs.AppendEmpty().Body().SetStr("log")
				wantRecords++
			}
		}

		batches := splitLogsByServiceName(ld)

		gotRecords, gotResources := 0, 0
		for key, b := range batches {
			gotRecords += b.LogRecordCount()
			gotResources += b.ResourceLogs().Len()
			require.NotEmpty(t, key, "no batch may use the empty key")
			for i := 0; i < b.ResourceLogs().Len(); i++ {
				res := b.ResourceLogs().At(i).Resource()
				if svc, ok := res.Attributes().Get("service.name"); ok && svc.Str() != "" {
					require.Equal(t, svc.Str(), key, "named resource must be keyed by service name")
				} else {
					require.Equal(t, identity.OfResource(res).String(), key, "unnamed resource must be keyed by identity")
				}
			}
		}
		require.Equal(t, wantRecords, gotRecords, "no log record is lost or duplicated")
		require.Equal(t, ld.ResourceLogs().Len(), gotResources, "no resource is lost or duplicated")

		again := splitLogsByServiceName(ld)
		require.Len(t, again, len(batches), "split is deterministic")
		for k := range batches {
			require.Contains(t, again, k)
		}
	})
}

func BenchmarkSplitLogsByServiceNameUnnamed(b *testing.B) {
	ld := plog.NewLogs()
	for i := range 100 {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		rl.Resource().Attributes().PutStr("k8s.pod.name", fmt.Sprintf("pod-%d", i))
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		for range 10 {
			lrs.AppendEmpty().Body().SetStr("benchmark log message")
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = splitLogsByServiceName(ld)
	}
}

func BenchmarkSplitLogsByServiceNameNamed(b *testing.B) {
	ld := plog.NewLogs()
	for i := range 100 {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		lrs := rl.ScopeLogs().AppendEmpty().LogRecords()
		for range 10 {
			lrs.AppendEmpty().Body().SetStr("benchmark log message")
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = splitLogsByServiceName(ld)
	}
}
