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
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
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
			_, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), tt.config)

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
				p, _ := newLogsExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())
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
				p, _ := newLogsExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())

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
			res := p.Start(t.Context(), componenttest.NewNopHost())
			defer func() {
				require.NoError(t, p.Shutdown(t.Context()))
			}()

			// verify
			require.Equal(t, tt.err, res)
		})
	}
}

func TestLogExporterShutdown(t *testing.T) {
	p, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(t.Context())

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
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	res := p.ConsumeLogs(t.Context(), simpleLogs())

	// verify
	assert.NoError(t, res)
}

func TestConsumeLogsEmitsOnlyParentExporterMetrics(t *testing.T) {
	ctx := t.Context()
	shutdownCtx := context.Background() //nolint:usetesting // Context must outlive test for cleanup
	telemetry := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, telemetry.Shutdown(shutdownCtx))
	})

	parentParams := exportertest.NewNopSettings(metadata.Type)
	parentParams.TelemetrySettings = telemetry.NewTelemetrySettings()

	cfg := simpleConfig()
	logsExporter, err := newLogsExporter(parentParams, cfg)
	require.NoError(t, err)

	otlpFactory := otlpexporter.NewFactory()
	var childSettings []exporter.Settings
	logsExporter.loadBalancer.componentFactory = func(createCtx context.Context, endpoint string) (component.Component, error) {
		childCfg := buildExporterConfig(cfg, endpoint)
		childParams := buildExporterSettings(otlpFactory.Type(), parentParams, endpoint)
		childSettings = append(childSettings, childParams)

		return exporterhelper.NewLogs(createCtx, childParams, &childCfg, func(context.Context, plog.Logs) error {
			return nil
		})
	}
	wrappedExporter, err := exporterhelper.NewLogs(
		ctx,
		parentParams,
		cfg,
		logsExporter.ConsumeLogs,
		exporterhelper.WithStart(logsExporter.Start),
		exporterhelper.WithShutdown(logsExporter.Shutdown),
		exporterhelper.WithCapabilities(logsExporter.Capabilities()),
	)
	require.NoError(t, err)

	require.NoError(t, wrappedExporter.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, wrappedExporter.Shutdown(ctx))
	})

	logs := generateSingleLogRecord()
	require.NoError(t, wrappedExporter.ConsumeLogs(ctx, logs))

	metric, err := telemetry.GetMetric("otelcol_exporter_sent_log_records")
	require.NoError(t, err)
	sum, ok := metric.Data.(metricdata.Sum[int64])
	require.True(t, ok)

	exporterKey := attribute.Key("exporter")
	var loadbalancingTotal int64
	for _, dp := range sum.DataPoints {
		attr, found := dp.Attributes.Value(exporterKey)
		require.True(t, found, "exporter attribute must be present")
		if attr.AsString() != parentParams.ID.String() {
			assert.Failf(t, "unexpected exporter attribute", "got %s", attr.AsString())
			continue
		}
		loadbalancingTotal += dp.Value
	}

	assert.Equal(t, int64(logs.LogRecordCount()), loadbalancingTotal)

	loadbalancerMetric, err := telemetry.GetMetric("otelcol_loadbalancer_backend_outcome")
	require.NoError(t, err)
	lbSum, ok := loadbalancerMetric.Data.(metricdata.Sum[int64])
	require.True(t, ok)
	var totalBackendOutcome int64
	for _, dp := range lbSum.DataPoints {
		totalBackendOutcome += dp.Value
	}
	assert.Equal(t, int64(1), totalBackendOutcome)

	require.Len(t, childSettings, 1)
	assert.IsType(t, metricnoop.NewMeterProvider(), childSettings[0].MeterProvider)
	assert.IsType(t, tracenoop.NewTracerProvider(), childSettings[0].TracerProvider)
}

func generateSingleLogRecord() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log")
	logRecord.SetTimestamp(pcommon.Timestamp(123))
	return logs
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
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	res := p.ConsumeLogs(t.Context(), simpleLogs())

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
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	first := simpleLogs()
	second := simpleLogWithID(pcommon.TraceID([16]byte{2, 3, 4, 5}))
	batch := plog.NewLogs()
	firstTgt := batch.ResourceLogs().AppendEmpty()
	first.ResourceLogs().At(0).CopyTo(firstTgt)
	secondTgt := batch.ResourceLogs().AppendEmpty()
	second.ResourceLogs().At(0).CopyTo(secondTgt)

	// test
	err = p.ConsumeLogs(t.Context(), batch)

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
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	err = p.ConsumeLogs(t.Context(), simpleLogWithoutID())

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 1)
}

// this test validates that exporter is can concurrently change the endpoints while consuming logs.
func TestConsumeLogs_ConcurrentResolverChange(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	consumeStarted := make(chan struct{})
	consumeDone := make(chan struct{})

	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		// imitate a slow exporter
		te := &mockLogsExporter{Component: mockComponent{}}
		te.consumelogsfn = func(_ context.Context, _ plog.Logs) error {
			close(consumeStarted)
			time.Sleep(50 * time.Millisecond)
			return te.consumeErr
		}
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	go func() {
		assert.NoError(t, p.ConsumeLogs(t.Context(), simpleLogs()))
		close(consumeDone)
	}()

	// update endpoint while consuming logs
	<-consumeStarted
	endpoints = []string{"endpoint-2"}
	endpoint, err := lb.res.resolve(t.Context())
	require.NoError(t, err)
	require.Equal(t, endpoints, endpoint)
	<-consumeDone
}

func TestRollingUpdatesWhenConsumeLogs(t *testing.T) {
	// this test is based on the discussion in the following issue for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	// prepare

	// simulate rolling updates, the dns resolver should resolve in the following order
	// ["127.0.0.1"] -> ["127.0.0.1", "127.0.0.2"] -> ["127.0.0.2"]
	ts, tb := getTelemetryAssets(t)
	res, err := newDNSResolver(zap.NewNop(), "service-1", "", 5*time.Second, 1*time.Second, nil, tb)
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
	res.resInterval = 100 * time.Millisecond

	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: ""}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(exportertest.NewNopSettings(metadata.Type), cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb

	counter1 := &atomic.Int64{}
	counter2 := &atomic.Int64{}
	id1 := "127.0.0.1:4317"
	id2 := "127.0.0.2:4317"
	unreachableCh := make(chan struct{})
	defaultExporters := map[string]*wrappedExporter{
		id1: newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			counter1.Add(1)
			counter.Add(1)
			// simulate an unreachable backend
			<-unreachableCh
			return nil
		}), id1),
		id2: newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			counter2.Add(1)
			return nil
		}), id2),
	}

	// test
	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
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

	ctx, cancel := context.WithCancel(t.Context())
	var waitWG sync.WaitGroup
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
				waitWG.Add(1)
				go func() {
					assert.NoError(t, p.ConsumeLogs(ctx, randomLogs()))
					waitWG.Done()
				}()
			}
		}
	}(ctx)

	// give limited but enough time to rolling updates. otherwise this test
	// will still pass due to the unreacheableCh that is used to simulate
	// unreachable backends.
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		require.Positive(tt, counter1.Load())
		require.Positive(tt, counter2.Load())
	}, 1*time.Second, 100*time.Millisecond)
	cancel()
	<-consumeCh

	// verify
	mu.Lock()
	require.Equal(t, []string{"127.0.0.2"}, lastResolved)
	mu.Unlock()

	close(unreachableCh)
	waitWG.Wait()
}

func TestConsumeLogs_DNSResolverRetriesOnUnreachableEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: "", Quarantine: QuarantineSettings{Enabled: true}}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Endpoint-1 returns an error to simulate an unreachable endpoint
	// Endpoint-2 succeeds normally
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			return nil
		}), "endpoint-2:4317"),
	}

	// test - first call should fail by retrying with endpoint-2
	res := p.ConsumeLogs(t.Context(), simpleLogs())
	require.NoError(t, res)

	// We want to verify that logs are consumed by "endpoint-2" exporter, which uses consumeWithRetry.
	// To do so, we can provide a spy function and check whether it is called.
	var consumedBySecondEndpoint atomic.Bool
	lb.exporters["endpoint-2:4317"] = newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
		consumedBySecondEndpoint.Store(true)
		return nil
	}), "endpoint-2:4317")

	// Re-run test with the spy.
	require.NoError(t, p.ConsumeLogs(t.Context(), simpleLogs()))
	require.True(t, consumedBySecondEndpoint.Load(), "logs should be consumed by the second endpoint")
}

func TestConsumeLogs_DNSResolverRetriesExhausted(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: "", Quarantine: QuarantineSettings{Enabled: true}}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Both endpoints return an error to simulate unreachable endpoints
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			return errors.New("endpoint unreachable")
		}), "endpoint-2:4317"),
	}

	// test - first call should fail by retrying with endpoint-2
	res := p.ConsumeLogs(t.Context(), simpleLogs())
	require.Error(t, res)
	require.EqualError(t, res, "all endpoints were tried and failed: map[endpoint-1:true endpoint-2:true]")
}

func TestConsumeLogs_DNSResolverQuarantineWithParentExporterBackoffEnabled(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{
				Hostname: "service-1",
				Port:     "",
				Quarantine: QuarantineSettings{
					Enabled:  true,
					Duration: 5 * time.Millisecond, // Short quarantine - must be shorter than backoff intervals
				},
			}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb

	// Track attempts per endpoint to verify quarantine logic tries all endpoints
	endpoint1Attempts := &atomic.Int64{}
	endpoint2Attempts := &atomic.Int64{}
	totalAttempts := &atomic.Int64{}
	var attemptTimes []time.Time
	var timesMutex sync.Mutex

	// Both endpoints return an error to simulate unreachable endpoints
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			endpoint1Attempts.Add(1)
			totalAttempts.Add(1)
			timesMutex.Lock()
			attemptTimes = append(attemptTimes, time.Now())
			timesMutex.Unlock()
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			endpoint2Attempts.Add(1)
			totalAttempts.Add(1)
			timesMutex.Lock()
			attemptTimes = append(attemptTimes, time.Now())
			timesMutex.Unlock()
			return errors.New("endpoint unreachable")
		}), "endpoint-2:4317"),
	}

	// Max elapsed time is 250ms, so it should retry 4 times per endpoint
	// 10ms, 20ms, 40ms, 80ms
	// Total time should be 150ms
	wrappedExporterBackOffConfig := configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     10 * time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          2,
		MaxInterval:         200 * time.Millisecond,
		MaxElapsedTime:      250 * time.Millisecond,
	}

	// Wrap with exporterhelper to enable retry logic with backoff
	retryExporter, err := exporterhelper.NewLogs(
		t.Context(),
		ts,
		cfg,
		p.ConsumeLogs,
		exporterhelper.WithStart(p.Start),
		exporterhelper.WithShutdown(p.Shutdown),
		exporterhelper.WithCapabilities(p.Capabilities()),
		exporterhelper.WithRetry(wrappedExporterBackOffConfig),
	)
	require.NoError(t, err)

	err = retryExporter.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, retryExporter.Shutdown(t.Context()))
	}()

	// Test: ConsumeLogs should fail after retrying all endpoints multiple times with backoff
	start := time.Now()
	res := retryExporter.ConsumeLogs(t.Context(), simpleLogs())
	elapsed := time.Since(start)

	// Verify that an error occurred
	require.Error(t, res)
	require.Contains(t, res.Error(), "all endpoints were tried and failed")

	t.Logf("Total attempts: %d, Endpoint-1: %d, Endpoint-2: %d, Elapsed: %v",
		totalAttempts.Load(), endpoint1Attempts.Load(), endpoint2Attempts.Load(), elapsed)

	// Verify quarantine logic: both endpoints should be tried
	// In the first cycle, quarantine logic should try both endpoints immediately
	require.Positive(t, endpoint1Attempts.Load(), "endpoint-1 should be tried at least once")
	require.Positive(t, endpoint2Attempts.Load(), "endpoint-2 should be tried at least once")

	// With backoff and quarantine working together:
	require.GreaterOrEqual(t, totalAttempts.Load(), int64(2),
		"quarantine logic should try both endpoints at least in the initial cycle")

	if totalAttempts.Load() > 2 {
		t.Logf("Backoff retry occurred: multiple retry cycles observed")

		// Verify that backoff timing was applied by checking the time between first and last attempts
		timesMutex.Lock()
		if len(attemptTimes) > 2 {
			firstAttempt := attemptTimes[0]
			lastAttempt := attemptTimes[len(attemptTimes)-1]
			timeBetween := lastAttempt.Sub(firstAttempt)
			t.Logf("Time between first and last attempt: %v", timeBetween)
			// We expect multiple retry cycles around ~150ms total elapsed time
			require.Greater(t, timeBetween.Milliseconds(), int64(125),
				"should have backoff delay between retry cycles")
		}
		timesMutex.Unlock()
	}

	// The total elapsed time should reflect backoff delays if retries occurred
	t.Logf("Total elapsed time: %v", elapsed)
}

func TestConsumeLogs_DNSResolverQuarantineWithParentExporterBackoffDisabled(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{
				Hostname: "service-1",
				Port:     "",
				Quarantine: QuarantineSettings{
					Enabled:  true,
					Duration: 5 * time.Millisecond, // Short quarantine - must be shorter than backoff intervals
				},
			}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb

	// Track attempts per endpoint to verify quarantine logic tries all endpoints
	endpoint1Attempts := &atomic.Int64{}
	endpoint2Attempts := &atomic.Int64{}
	totalAttempts := &atomic.Int64{}
	var attemptTimes []time.Time
	var timesMutex sync.Mutex

	// Both endpoints return an error to simulate unreachable endpoints
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			endpoint1Attempts.Add(1)
			totalAttempts.Add(1)
			timesMutex.Lock()
			attemptTimes = append(attemptTimes, time.Now())
			timesMutex.Unlock()
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			endpoint2Attempts.Add(1)
			totalAttempts.Add(1)
			timesMutex.Lock()
			attemptTimes = append(attemptTimes, time.Now())
			timesMutex.Unlock()
			return errors.New("endpoint unreachable")
		}), "endpoint-2:4317"),
	}

	// Wrap with exporterhelper without backoff
	retryExporter, err := exporterhelper.NewLogs(
		t.Context(),
		ts,
		cfg,
		p.ConsumeLogs,
		exporterhelper.WithStart(p.Start),
		exporterhelper.WithShutdown(p.Shutdown),
		exporterhelper.WithCapabilities(p.Capabilities()),
	)
	require.NoError(t, err)

	err = retryExporter.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, retryExporter.Shutdown(t.Context()))
	}()

	// Test: ConsumeLogs should fail after retrying all endpoints multiple times with backoff
	start := time.Now()
	res := retryExporter.ConsumeLogs(t.Context(), simpleLogs())
	elapsed := time.Since(start)

	// Verify that an error occurred
	require.Error(t, res)
	require.Contains(t, res.Error(), "all endpoints were tried and failed")

	t.Logf("Total attempts: %d, Endpoint-1: %d, Endpoint-2: %d, Elapsed: %v",
		totalAttempts.Load(), endpoint1Attempts.Load(), endpoint2Attempts.Load(), elapsed)

	// Verify quarantine logic: both endpoints should be tried
	require.Equal(t, int64(1), endpoint1Attempts.Load(), "endpoint-1 should be tried once")
	require.Equal(t, int64(1), endpoint2Attempts.Load(), "endpoint-2 should be tried once")
	require.Equal(t, int64(2), totalAttempts.Load(),
		"quarantine logic should try both endpoints once")
}

// TestConsumeLogs_DNSResolverQuarantineWithParentExporterBackoffConfig_PartialRecovery tests the scenario
// where some endpoints recover after being quarantined
func TestConsumeLogs_DNSResolverQuarantineWithParentExporterBackoffEnabled_PartialRecovery(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{
				Hostname: "service-1",
				Port:     "",
				Quarantine: QuarantineSettings{
					Enabled:  true,
					Duration: 100 * time.Millisecond, // Short quarantine for testing
				},
			}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb

	// Endpoint-1 fails initially but recovers after 150ms
	// Endpoint-2 always fails
	endpoint1Attempts := &atomic.Int64{}
	endpoint2Attempts := &atomic.Int64{}
	var startTime time.Time

	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			endpoint1Attempts.Add(1)
			// Simulate recovery after 150ms
			if time.Since(startTime) > 150*time.Millisecond {
				t.Log("endpoint-1 recovered")
				return nil
			}
			t.Log("endpoint-1 temporarily unreachable")
			return errors.New("endpoint temporarily unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			endpoint2Attempts.Add(1)
			return errors.New("endpoint unreachable")
		}), "endpoint-2:4317"),
	}

	wrappedExporterBackOffConfig := configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     10 * time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          2,
		MaxInterval:         200 * time.Millisecond,
		MaxElapsedTime:      250 * time.Millisecond,
	}

	// Wrap with exporterhelper to enable retry logic with backoff
	retryExporter, err := exporterhelper.NewLogs(
		t.Context(),
		ts,
		cfg,
		p.ConsumeLogs,
		exporterhelper.WithStart(p.Start),
		exporterhelper.WithShutdown(p.Shutdown),
		exporterhelper.WithCapabilities(p.Capabilities()),
		exporterhelper.WithRetry(wrappedExporterBackOffConfig),
	)
	require.NoError(t, err)

	err = retryExporter.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, retryExporter.Shutdown(t.Context()))
	}()

	// Test: ConsumeLogs should eventually succeed when endpoint-1 recovers
	// Set startTime just before the call to ensure consistent timing across platforms
	startTime = time.Now()
	res := retryExporter.ConsumeLogs(t.Context(), simpleLogs())

	// Should succeed after endpoint-1 recovers
	require.NoError(t, res, "should succeed after endpoint-1 recovers")

	t.Logf("Endpoint-1 attempts: %d, Endpoint-2 attempts: %d",
		endpoint1Attempts.Load(), endpoint2Attempts.Load())

	// Both endpoints should have been tried (quarantine logic)
	require.Greater(t, endpoint1Attempts.Load(), int64(1), "endpoint-1 should be retried after quarantine period")
	require.Positive(t, endpoint2Attempts.Load(), "endpoint-2 should be tried at least once")
}

func TestConsumeLogs_DNSResolverQuarantineWithSubExporterBackoffEnabled(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{
				Hostname: "service-1",
				Port:     "",
				Quarantine: QuarantineSettings{
					Enabled:  true,
					Duration: 60 * time.Second, // Long quarantine - must be longer than backoff intervals
				},
			}),
		},
		Protocol: Protocol{
			OTLP: otlpexporter.Config{
				// Max elapsed time is 250ms, so it should retry 4 times per endpoint
				RetryConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Millisecond,
					RandomizationFactor: 0,
					Multiplier:          2,
					MaxInterval:         200 * time.Millisecond,
					MaxElapsedTime:      250 * time.Millisecond,
				},
			},
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb

	// Track attempts per endpoint to verify quarantine logic tries all endpoints
	endpoint1Attempts := &atomic.Int64{}
	endpoint2Attempts := &atomic.Int64{}
	totalAttempts := &atomic.Int64{}
	var attemptTimes []time.Time
	var timesMutex sync.Mutex

	// Both endpoints return an error to simulate unreachable endpoints
	// Create mock exporters with retry logic applied via exporterhelper
	mockExporter1 := newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
		endpoint1Attempts.Add(1)
		totalAttempts.Add(1)
		timesMutex.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		timesMutex.Unlock()
		return errors.New("endpoint unreachable")
	})
	mockExporter2 := newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
		endpoint2Attempts.Add(1)
		totalAttempts.Add(1)
		timesMutex.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		timesMutex.Unlock()
		return errors.New("endpoint unreachable")
	})

	// Wrap with exporterhelper to apply retry configuration from cfg.Protocol.OTLP.RetryConfig
	wrappedExp1, err := exporterhelper.NewLogs(
		t.Context(),
		exporter.Settings{
			ID:                component.NewIDWithName(component.MustNewType("otlp"), "endpoint-1"),
			TelemetrySettings: ts.TelemetrySettings,
		},
		&cfg.Protocol.OTLP,
		mockExporter1.ConsumeLogs,
		exporterhelper.WithRetry(cfg.Protocol.OTLP.RetryConfig),
		exporterhelper.WithStart(mockExporter1.Start),
		exporterhelper.WithShutdown(mockExporter1.Shutdown),
	)
	require.NoError(t, err)

	wrappedExp2, err := exporterhelper.NewLogs(
		t.Context(),
		exporter.Settings{
			ID:                component.NewIDWithName(component.MustNewType("otlp"), "endpoint-2"),
			TelemetrySettings: ts.TelemetrySettings,
		},
		&cfg.Protocol.OTLP,
		mockExporter2.ConsumeLogs,
		exporterhelper.WithRetry(cfg.Protocol.OTLP.RetryConfig),
		exporterhelper.WithStart(mockExporter2.Start),
		exporterhelper.WithShutdown(mockExporter2.Shutdown),
	)
	require.NoError(t, err)

	// Add the wrapped exporters to the load balancer
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(wrappedExp1, "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(wrappedExp2, "endpoint-2:4317"),
	}

	host := componenttest.NewNopHost()

	// Start the load balancer exporter
	err = p.Start(t.Context(), host)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Test: ConsumeLogs should fail after retrying all endpoints multiple times with backoff
	start := time.Now()
	res := p.ConsumeLogs(t.Context(), simpleLogs())
	elapsed := time.Since(start)

	// Verify that an error occurred
	require.Error(t, res)
	require.Contains(t, res.Error(), "all endpoints were tried and failed")

	t.Logf("Total attempts: %d, Endpoint-1: %d, Endpoint-2: %d, Elapsed: %v",
		totalAttempts.Load(), endpoint1Attempts.Load(), endpoint2Attempts.Load(), elapsed)

	// Verify quarantine logic: both endpoints should be tried
	// In the first cycle, quarantine logic should try both endpoints immediately
	require.Positive(t, endpoint1Attempts.Load(), "endpoint-1 should be tried at least once")
	require.Positive(t, endpoint2Attempts.Load(), "endpoint-2 should be tried at least once")

	// With backoff and quarantine working together:
	require.GreaterOrEqual(t, totalAttempts.Load(), int64(2),
		"quarantine logic should try both endpoints at least in the initial cycle")

	if totalAttempts.Load() > 2 {
		t.Logf("Backoff retry occurred: multiple retry cycles observed")

		// Verify that backoff timing was applied by checking the time between first and last attempts
		timesMutex.Lock()
		if len(attemptTimes) > 2 {
			firstAttempt := attemptTimes[0]
			lastAttempt := attemptTimes[len(attemptTimes)-1]
			timeBetween := lastAttempt.Sub(firstAttempt)
			t.Logf("Time between first and last attempt: %v", timeBetween)
			// we expect multiple retry cycles around ~310ms total elapsed time
			require.Greater(t, timeBetween.Milliseconds(), int64(275),
				"should have backoff delay between retry cycles")
		}
		timesMutex.Unlock()
	}

	// The total elapsed time should reflect backoff delays if retries occurred
	t.Logf("Total elapsed time: %v", elapsed)
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

func (*mockLogsExporter) Capabilities() consumer.Capabilities {
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
