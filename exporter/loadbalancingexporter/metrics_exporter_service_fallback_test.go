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
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

// metricDelivery records which endpoint received which resource.
type metricDelivery struct {
	mu sync.Mutex
	// points counts data points per endpoint.
	points map[string]int
	// endpointByHost maps the host.name resource attribute to the endpoints that received it.
	endpointByHost map[string]map[string]struct{}
	// endpointBySvc maps the service.name resource attribute to the endpoints that received it.
	endpointBySvc map[string]map[string]struct{}
}

func newMetricDelivery() *metricDelivery {
	return &metricDelivery{
		points:         map[string]int{},
		endpointByHost: map[string]map[string]struct{}{},
		endpointBySvc:  map[string]map[string]struct{}{},
	}
}

func (d *metricDelivery) record(endpoint string, md pmetric.Metrics) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.points[endpoint] += md.DataPointCount()
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		attrs := md.ResourceMetrics().At(i).Resource().Attributes()
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

// newServiceRoutingMetricsExporter builds the real metrics exporter and the real load
// balancer with a hash ring over the endpoints. Only the per-endpoint sub-exporter is a mock.
func newServiceRoutingMetricsExporter(t *testing.T, routingKey string, endpoints []string, d *metricDelivery) *metricExporterImp {
	t.Helper()
	ts, tb := getTelemetryAssets(t)

	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, md pmetric.Metrics) error {
			d.record(endpoint, md)
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: routingKey,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, cfg)
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
	t.Cleanup(func() { require.NoError(t, p.Shutdown(context.Background())) })
	return p
}

// resourceMetric returns one resource with one gauge data point. Empty attribute values are skipped.
func resourceMetric(service, host string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	if service != "" {
		rm.Resource().Attributes().PutStr("service.name", service)
	}
	rm.Resource().Attributes().PutStr("host.name", host)
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test.gauge")
	m.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1)
	return md
}

// TestE2EMissingServiceNameSpreadsAcrossBackends is an end-to-end check through the real
// exporter and the real hash ring. It proves that resources without service.name
//  1. are delivered and not dropped,
//  2. are not all sent to one backend,
//  3. always reach the same backend (per-resource affinity).
func TestE2EMissingServiceNameSpreadsAcrossBackends(t *testing.T) {
	const (
		resources = 90
		rounds    = 3
	)
	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	d := newMetricDelivery()
	p := newServiceRoutingMetricsExporter(t, svcRoutingStr, endpoints, d)

	for round := range rounds {
		for i := range resources {
			require.NoError(t, p.ConsumeMetrics(t.Context(), resourceMetric("", fmt.Sprintf("host-%d", i))), "round %d", round)
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	total := 0
	for _, n := range d.points {
		total += n
	}
	assert.Equal(t, resources*rounds, total, "every data point is delivered exactly once")

	t.Logf("data points per endpoint (%d resources x %d rounds): %v", resources, rounds, d.points)
	assert.Len(t, d.points, len(endpoints), "every endpoint receives data")
	for ep, n := range d.points {
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
	svcDelivery, resDelivery := newMetricDelivery(), newMetricDelivery()
	svcExporter := newServiceRoutingMetricsExporter(t, svcRoutingStr, endpoints, svcDelivery)
	resExporter := newServiceRoutingMetricsExporter(t, resourceRoutingStr, endpoints, resDelivery)

	for i := range 60 {
		md := resourceMetric("", fmt.Sprintf("host-%d", i))
		require.NoError(t, svcExporter.ConsumeMetrics(t.Context(), md))
		require.NoError(t, resExporter.ConsumeMetrics(t.Context(), md))
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
	d := newMetricDelivery()
	p := newServiceRoutingMetricsExporter(t, svcRoutingStr, endpoints, d)

	services := []string{"payment", "auth", "order", "inventory", "shipping", "notify"}
	for _, svc := range services {
		for i := range 20 {
			require.NoError(t, p.ConsumeMetrics(t.Context(), resourceMetric(svc, fmt.Sprintf("%s-host-%d", svc, i))))
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	for _, svc := range services {
		assert.Len(t, d.endpointBySvc[svc], 1, "service %s must stay on one backend", svc)
	}
}

// TestE2EMixedBatchDeliversEveryPointOnce sends named, unnamed and empty-name resources in
// one batch. Before the change the unnamed ones were dropped. Now no data point is lost
// or duplicated.
func TestE2EMixedBatchDeliversEveryPointOnce(t *testing.T) {
	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	d := newMetricDelivery()
	p := newServiceRoutingMetricsExporter(t, svcRoutingStr, endpoints, d)

	md := pmetric.NewMetrics()
	want := 0
	for i := range 30 {
		rm := md.ResourceMetrics().AppendEmpty()
		switch i % 3 {
		case 0:
			rm.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		case 1:
			rm.Resource().Attributes().PutStr("service.name", "")
		}
		rm.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("test.gauge")
		dps := m.SetEmptyGauge().DataPoints()
		for range 2 {
			dps.AppendEmpty().SetDoubleValue(1)
			want++
		}
	}
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))

	d.mu.Lock()
	defer d.mu.Unlock()
	got := 0
	for _, n := range d.points {
		got += n
	}
	assert.Equal(t, want, got)
}

func TestSplitMetricsByServiceNameFallback(t *testing.T) {
	t.Run("unnamed resource is keyed by resource identity", func(t *testing.T) {
		md := resourceMetric("", "host-1")
		batches := splitMetricsByResourceServiceName(md)
		require.Len(t, batches, 1)
		want := identity.OfResource(md.ResourceMetrics().At(0).Resource()).String()
		require.Contains(t, batches, want)
		assert.NotEmpty(t, want)
	})

	t.Run("empty service name counts as missing", func(t *testing.T) {
		md := resourceMetric("", "host-1")
		md.ResourceMetrics().At(0).Resource().Attributes().PutStr("service.name", "")
		batches := splitMetricsByResourceServiceName(md)
		require.Len(t, batches, 1)
		assert.NotContains(t, batches, "")
	})

	t.Run("different unnamed resources get different keys", func(t *testing.T) {
		md := pmetric.NewMetrics()
		for i := range 5 {
			resourceMetric("", fmt.Sprintf("host-%d", i)).ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
		}
		assert.Len(t, splitMetricsByResourceServiceName(md), 5)
	})

	t.Run("identical unnamed resources are merged into one batch", func(t *testing.T) {
		md := pmetric.NewMetrics()
		for range 3 {
			resourceMetric("", "host-1").ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
		}
		batches := splitMetricsByResourceServiceName(md)
		require.Len(t, batches, 1)
		for _, b := range batches {
			assert.Equal(t, 3, b.DataPointCount())
		}
	})

	t.Run("key is deterministic", func(t *testing.T) {
		a := splitMetricsByResourceServiceName(resourceMetric("", "host-1"))
		b := splitMetricsByResourceServiceName(resourceMetric("", "host-1"))
		for k := range a {
			assert.Contains(t, b, k)
		}
	})

	t.Run("named resources keep the service name as key", func(t *testing.T) {
		batches := splitMetricsByResourceServiceName(resourceMetric("svc-1", "host-1"))
		require.Len(t, batches, 1)
		assert.Contains(t, batches, "svc-1")
	})
}

// FuzzSplitMetricsByResourceServiceName checks the invariants of the split for arbitrary
// resource layouts. Each input byte describes one resource.
func FuzzSplitMetricsByResourceServiceName(f *testing.F) {
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
		md := pmetric.NewMetrics()
		wantPoints := 0
		for _, b := range data {
			rm := md.ResourceMetrics().AppendEmpty()
			attrs := rm.Resource().Attributes()
			switch b % 3 {
			case 0: // named
				attrs.PutStr("service.name", fmt.Sprintf("svc-%d", b>>2))
			case 1: // empty service name
				attrs.PutStr("service.name", "")
			default: // missing
			}
			attrs.PutStr("host.name", fmt.Sprintf("host-%d", b>>4))
			m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
			m.SetName("test.gauge")
			dps := m.SetEmptyGauge().DataPoints()
			n := int(b>>6) + 1
			for range n {
				dps.AppendEmpty().SetDoubleValue(1)
				wantPoints++
			}
		}

		batches := splitMetricsByResourceServiceName(md)

		gotPoints, gotResources := 0, 0
		for key, b := range batches {
			gotPoints += b.DataPointCount()
			gotResources += b.ResourceMetrics().Len()
			require.NotEmpty(t, key, "no batch may use the empty key")
			for i := 0; i < b.ResourceMetrics().Len(); i++ {
				res := b.ResourceMetrics().At(i).Resource()
				if svc, ok := res.Attributes().Get("service.name"); ok && svc.Str() != "" {
					require.Equal(t, svc.Str(), key, "named resource must be keyed by service name")
				} else {
					require.Equal(t, identity.OfResource(res).String(), key, "unnamed resource must be keyed by identity")
				}
			}
		}
		require.Equal(t, wantPoints, gotPoints, "no data point is lost or duplicated")
		// Identical resources are merged within a batch, so there can be fewer resources.
		require.LessOrEqual(t, gotResources, md.ResourceMetrics().Len(), "no resource is duplicated")
		if md.ResourceMetrics().Len() > 0 {
			require.NotEmpty(t, batches, "no resource is lost")
		}

		again := splitMetricsByResourceServiceName(md)
		require.Len(t, again, len(batches), "split is deterministic")
		for k := range batches {
			require.Contains(t, again, k)
		}
	})
}

func benchmarkMetrics(named bool) pmetric.Metrics {
	md := pmetric.NewMetrics()
	for i := range 100 {
		rm := md.ResourceMetrics().AppendEmpty()
		if named {
			rm.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		}
		rm.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		rm.Resource().Attributes().PutStr("k8s.pod.name", fmt.Sprintf("pod-%d", i))
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("test.gauge")
		dps := m.SetEmptyGauge().DataPoints()
		for range 10 {
			dps.AppendEmpty().SetDoubleValue(1)
		}
	}
	return md
}

func BenchmarkSplitMetricsByResourceServiceNameUnnamed(b *testing.B) {
	md := benchmarkMetrics(false)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = splitMetricsByResourceServiceName(md)
	}
}

func BenchmarkSplitMetricsByResourceServiceNameNamed(b *testing.B) {
	md := benchmarkMetrics(true)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = splitMetricsByResourceServiceName(md)
	}
}
