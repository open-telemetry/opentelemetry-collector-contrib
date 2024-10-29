// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestNewLogsExporter(t *testing.T) {
	for _, tt := range []struct {
		desc   string
		config *Config
		err    error
	}{
		{
			"simple",
			simpleConfig(),
			nil,
		},
		{
			"empty",
			&Config{},
			errNoResolver,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// test
			_, err := newLogsExporter(exportertest.NewNopSettings(), tt.config)

			// verify
			require.Equal(t, tt.err, err)
		})
	}
}

func TestLogExporterStart(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	for _, tt := range []struct {
		desc string
		le   *logExporterImp
		err  error
	}{
		{
			"ok",
			func() *logExporterImp {
				p, _ := newLogsExporter(exportertest.NewNopSettings(), simpleConfig())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *logExporterImp {
				// prepare
				lb, err := newLoadBalancer(ts.Logger, simpleConfig(), nil, tb)
				require.NoError(t, err)
				p, _ := newLogsExporter(exportertest.NewNopSettings(), simpleConfig())

				lb.res = &mockResolver{
					onStart: func(context.Context) error {
						return errors.New("some expected err")
					},
				}
				p.loadBalancer = lb

				return p
			}(),
			errors.New("some expected err"),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			p := tt.le

			// test
			res := p.Start(context.Background(), componenttest.NewNopHost())
			defer func() {
				require.NoError(t, p.Shutdown(context.Background()))
			}()

			// verify
			require.Equal(t, tt.err, res)
		})
	}
}

func TestLogExporterShutdown(t *testing.T) {
	p, err := newLogsExporter(exportertest.NewNopSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.NoError(t, res)
}

func TestConsumeLogs(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}

	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeLogs(context.Background(), simpleLogs())

	// verify
	assert.NoError(t, res)
}

func TestConsumeLogsUnexpectedExporterType(t *testing.T) {
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	ts, tb := getTelemetryAssets(t)

	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeLogs(context.Background(), simpleLogs())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("unable to export logs, unexpected exporter type: expected exporter.Logs but got %T", newNopMockExporter()))
}

func TestLogBatchWithTwoTraces(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}

	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	first := simpleLogs()
	second := simpleLogWithID(pcommon.TraceID([16]byte{2, 3, 4, 5}))
	batch := plog.NewLogs()
	firstTgt := batch.ResourceLogs().AppendEmpty()
	first.ResourceLogs().At(0).CopyTo(firstTgt)
	secondTgt := batch.ResourceLogs().AppendEmpty()
	second.ResourceLogs().At(0).CopyTo(secondTgt)

	// test
	err = p.ConsumeLogs(context.Background(), batch)

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 2)
}

func TestNoLogsInBatch(t *testing.T) {
	for _, tt := range []struct {
		desc  string
		batch plog.Logs
	}{
		{
			"no resource logs",
			plog.NewLogs(),
		},
		{
			"no instrumentation library logs",
			func() plog.Logs {
				batch := plog.NewLogs()
				batch.ResourceLogs().AppendEmpty()
				return batch
			}(),
		},
		{
			"no logs",
			func() plog.Logs {
				batch := plog.NewLogs()
				batch.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty()
				return batch
			}(),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res := traceIDFromLogs(tt.batch)
			assert.Equal(t, pcommon.NewTraceIDEmpty(), res)
		})
	}
}

func TestLogsWithoutTraceID(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	err = p.ConsumeLogs(context.Background(), simpleLogWithoutID())

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 1)
}

// this test validates that exporter is can concurrently change the endpoints while consuming logs.
func TestConsumeLogs_ConcurrentResolverChange(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	consumeStarted := make(chan struct{})
	consumeDone := make(chan struct{})

	// imitate a slow exporter
	te := &mockLogsExporter{Component: mockComponent{}}
	te.consumelogsfn = func(_ context.Context, _ plog.Logs) error {
		close(consumeStarted)
		time.Sleep(50 * time.Millisecond)
		return te.consumeErr
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return te, nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	endpoints := []string{"endpoint-1"}
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	go func() {
		assert.NoError(t, p.ConsumeLogs(context.Background(), simpleLogs()))
		close(consumeDone)
	}()

	// update endpoint while consuming logs
	<-consumeStarted
	endpoints = []string{"endpoint-2"}
	endpoint, err := lb.res.resolve(context.Background())
	require.NoError(t, err)
	require.Equal(t, endpoints, endpoint)
	<-consumeDone
}

func TestRollingUpdatesWhenConsumeLogs(t *testing.T) {
	t.Skip("Flaky Test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/13331")

	// this test is based on the discussion in the following issue for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	// prepare

	// simulate rolling updates, the dns resolver should resolve in the following order
	// ["127.0.0.1"] -> ["127.0.0.1", "127.0.0.2"] -> ["127.0.0.2"]
	ts, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", 5*time.Second, 1*time.Second, tb)
	require.NoError(t, err)

	mu := sync.Mutex{}
	var lastResolved []string
	res.onChange(func(s []string) {
		mu.Lock()
		lastResolved = s
		mu.Unlock()
	})

	resolverCh := make(chan struct{}, 1)
	counter := &atomic.Int64{}
	resolve := [][]net.IPAddr{
		{
			{IP: net.IPv4(127, 0, 0, 1)},
		}, {
			{IP: net.IPv4(127, 0, 0, 1)},
			{IP: net.IPv4(127, 0, 0, 2)},
		}, {
			{IP: net.IPv4(127, 0, 0, 2)},
		},
	}
	res.resolver = &mockDNSResolver{
		onLookupIPAddr: func(context.Context, string) ([]net.IPAddr, error) {
			defer func() {
				counter.Add(1)
			}()

			if counter.Load() <= 2 {
				return resolve[counter.Load()], nil
			}

			if counter.Load() == 3 {
				// stop as soon as rolling updates end
				resolverCh <- struct{}{}
			}

			return resolve[2], nil
		},
	}
	res.resInterval = 10 * time.Millisecond

	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: &DNSResolver{Hostname: "service-1", Port: ""},
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(exportertest.NewNopSettings(), cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb

	counter1 := &atomic.Int64{}
	counter2 := &atomic.Int64{}
	id1 := "127.0.0.1:4317"
	id2 := "127.0.0.2:4317"
	defaultExporters := map[string]*wrappedExporter{
		id1: newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			counter1.Add(1)
			// simulate an unreachable backend
			time.Sleep(10 * time.Second)
			return nil
		}), id1),
		id2: newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			counter2.Add(1)
			return nil
		}), id2),
	}

	// test
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()
	// ensure using default exporters
	lb.updateLock.Lock()
	lb.exporters = defaultExporters
	lb.updateLock.Unlock()
	lb.res.onChange(func(_ []string) {
		lb.updateLock.Lock()
		lb.exporters = defaultExporters
		lb.updateLock.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	// keep consuming traces every 2ms
	consumeCh := make(chan struct{})
	go func(ctx context.Context) {
		ticker := time.NewTicker(2 * time.Millisecond)
		for {
			select {
			case <-ctx.Done():
				consumeCh <- struct{}{}
				return
			case <-ticker.C:
				go func() {
					assert.NoError(t, p.ConsumeLogs(ctx, randomLogs()))
				}()
			}
		}
	}(ctx)

	// give limited but enough time to rolling updates. otherwise this test
	// will still pass due to the 10 secs of sleep that is used to simulate
	// unreachable backends.
	go func() {
		time.Sleep(1 * time.Second)
		resolverCh <- struct{}{}
	}()

	<-resolverCh
	cancel()
	<-consumeCh

	// verify
	mu.Lock()
	require.Equal(t, []string{"127.0.0.2"}, lastResolved)
	mu.Unlock()
	require.Positive(t, counter1.Load())
	require.Positive(t, counter2.Load())
}

func randomLogs() plog.Logs {
	return simpleLogWithID(random())
}

func simpleLogs() plog.Logs {
	return simpleLogWithID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
}

func simpleLogWithID(id pcommon.TraceID) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty().SetTraceID(id)

	return logs
}

func simpleLogWithoutID() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty()

	return logs
}

type mockLogsExporter struct {
	component.Component
	consumelogsfn func(ctx context.Context, ld plog.Logs) error
	consumeErr    error
}

func (e *mockLogsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockLogsExporter) Shutdown(context.Context) error {
	e.consumeErr = errors.New("exporter is shut down")
	return nil
}

func (e *mockLogsExporter) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if e.consumelogsfn == nil {
		return e.consumeErr
	}
	return e.consumelogsfn(ctx, ld)
}

type mockComponent struct {
	component.StartFunc
	component.ShutdownFunc
}

func newMockLogsExporter(consumelogsfn func(ctx context.Context, ld plog.Logs) error) exporter.Logs {
	return &mockLogsExporter{
		Component:     mockComponent{},
		consumelogsfn: consumelogsfn,
	}
}

func newNopMockLogsExporter() exporter.Logs {
	return &mockLogsExporter{Component: mockComponent{}}
}
