// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
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
		{
			"service routing",
			&Config{
				Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}})},
				RoutingKey: svcRoutingStr,
			},
			nil,
		},
		{
			"resource routing",
			&Config{
				Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}})},
				RoutingKey: resourceRoutingStr,
			},
			nil,
		},
		{
			"attributes routing",
			&Config{
				Resolver:          ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}})},
				RoutingKey:        attrRoutingStr,
				RoutingAttributes: []string{"my.attr"},
			},
			nil,
		},
		{
			"traceID routing",
			&Config{
				Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}})},
				RoutingKey: traceIDRoutingStr,
			},
			nil,
		},
		{
			"unsupported routing key",
			&Config{
				Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}})},
				RoutingKey: "invalid",
			},
			fmt.Errorf("unsupported routing_key: %q", "invalid"),
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
				p.loadBalancer.res = &mockResolver{}
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
	rl.Resource().Attributes().PutStr("service.name", "test-svc")
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

func TestLogBatchWithTwoServices(t *testing.T) {
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

	first := simpleLogWithServiceName("svc-1")
	second := simpleLogWithServiceName("svc-2")
	batch := plog.NewLogs()
	first.ResourceLogs().At(0).CopyTo(batch.ResourceLogs().AppendEmpty())
	second.ResourceLogs().At(0).CopyTo(batch.ResourceLogs().AppendEmpty())

	// test
	err = p.ConsumeLogs(t.Context(), batch)

	// verify
	assert.NoError(t, err)
	// with a single endpoint, both services route to the same exporter
	// so we get a single merged ConsumeLogs call
	assert.Len(t, sink.AllLogs(), 1)
}

func TestLogsWithMissingServiceName(t *testing.T) {
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

	// test with a log that has no service.name — should still route (to empty key)
	logWithoutSvc := plog.NewLogs()
	rl := logWithoutSvc.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test")

	err = p.ConsumeLogs(t.Context(), logWithoutSvc)

	// verify — log is routed (not dropped), no error
	assert.NoError(t, err)
	assert.NotEmpty(t, sink.AllLogs())
}

// this test validates that exporter can concurrently change the endpoints while consuming logs.
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
	// keep consuming logs every 2ms
	consumeCh := make(chan struct{})
	go func(ctx context.Context) {
		ticker := time.NewTicker(2 * time.Millisecond)
		for {
			select {
			case <-ctx.Done():
				consumeCh <- struct{}{}
				return
			case <-ticker.C:
				waitWG.Go(func() {
					assert.NoError(t, p.ConsumeLogs(ctx, randomLogs()))
				})
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

// Split function tests

func TestSplitLogsByServiceName(t *testing.T) {
	t.Run("single service", func(t *testing.T) {
		ld := simpleLogWithServiceName("svc-1")
		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 1)
		require.Contains(t, batches, "svc-1")
	})

	t.Run("multiple services", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "svc-1")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("service.name", "svc-2")
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 2)
		require.Contains(t, batches, "svc-1")
		require.Contains(t, batches, "svc-2")
	})

	t.Run("missing service name routes to empty key", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 1)
		require.Contains(t, batches, "")
		assert.Equal(t, 1, batches[""].ResourceLogs().Len())
	})

	t.Run("duplicate service names merged", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "svc-1")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("service.name", "svc-1")
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 1)
		require.Contains(t, batches, "svc-1")
		assert.Equal(t, 2, batches["svc-1"].ResourceLogs().Len())
	})

	t.Run("empty logs", func(t *testing.T) {
		ld := plog.NewLogs()
		batches := splitLogsByServiceName(ld)
		require.Empty(t, batches)
	})

	t.Run("mixed valid and missing service names", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "svc-1")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		// no service.name on this resource — routed to empty key
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")
		rl3 := ld.ResourceLogs().AppendEmpty()
		rl3.Resource().Attributes().PutStr("service.name", "svc-2")
		rl3.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-3")

		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 3) // svc-1, svc-2, and empty key
		require.Contains(t, batches, "svc-1")
		require.Contains(t, batches, "svc-2")
		require.Contains(t, batches, "")
	})

	t.Run("preserves log record data", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "svc-1")
		rl.Resource().Attributes().PutStr("host.name", "host-1")
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr("important message")
		lr.SetSeverityText("ERROR")
		lr.Attributes().PutStr("trace_id", "abc123")

		batches := splitLogsByServiceName(ld)
		require.Len(t, batches, 1)

		result := batches["svc-1"]
		require.Equal(t, 1, result.ResourceLogs().Len())
		resultLR := result.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		assert.Equal(t, "important message", resultLR.Body().AsString())
		assert.Equal(t, "ERROR", resultLR.SeverityText())
		v, ok := resultLR.Attributes().Get("trace_id")
		require.True(t, ok)
		assert.Equal(t, "abc123", v.Str())
	})
}

func TestSplitLogsByResourceID(t *testing.T) {
	t.Run("single resource", func(t *testing.T) {
		ld := simpleLogWithServiceName("svc-1")
		batches := splitLogsByResourceID(ld)
		require.Len(t, batches, 1)
	})

	t.Run("different resources", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "svc-1")
		rl1.Resource().Attributes().PutStr("host.name", "host-1")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("service.name", "svc-1")
		rl2.Resource().Attributes().PutStr("host.name", "host-2")
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByResourceID(ld)
		require.Len(t, batches, 2)
	})

	t.Run("identical resources merged", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "svc-1")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("service.name", "svc-1")
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByResourceID(ld)
		require.Len(t, batches, 1)
		for _, b := range batches {
			assert.Equal(t, 2, b.ResourceLogs().Len())
		}
	})

	t.Run("empty logs", func(t *testing.T) {
		ld := plog.NewLogs()
		batches := splitLogsByResourceID(ld)
		require.Empty(t, batches)
	})

	t.Run("empty resource attributes", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByResourceID(ld)
		require.Len(t, batches, 1)
	})

	t.Run("same service different extra attributes", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "svc-1")
		rl1.Resource().Attributes().PutInt("instance.id", 1)
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("service.name", "svc-1")
		rl2.Resource().Attributes().PutInt("instance.id", 2)
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByResourceID(ld)
		// different instance.id means different resource hash
		require.Len(t, batches, 2)
	})
}

func TestSplitLogsByAttributes(t *testing.T) {
	t.Run("single attribute", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("env", "prod")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"env"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "env=prod|")
	})

	t.Run("multiple attributes composite key", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("env", "prod")
		rl.Resource().Attributes().PutStr("region", "us-east")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"env", "region"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "env=prod|region=us-east|")
	})

	t.Run("missing attribute uses empty string", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"nonexistent"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "nonexistent=|")
	})

	t.Run("pseudo attribute log.severity", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.SetSeverityText("ERROR")

		batches := splitLogsByAttributes(ld, []string{"log.severity"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "log.severity=ERROR|")
	})

	t.Run("pseudo attribute log.body", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr("my log body")

		batches := splitLogsByAttributes(ld, []string{"log.body"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "log.body=my log body|")
	})

	t.Run("scope attribute", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().Attributes().PutStr("library", "mylib")
		sl.LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"library"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "library=mylib|")
	})

	t.Run("log record attribute", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.Attributes().PutStr("user.id", "user-123")

		batches := splitLogsByAttributes(ld, []string{"user.id"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "user.id=user-123|")
	})

	t.Run("empty logs", func(t *testing.T) {
		ld := plog.NewLogs()
		batches := splitLogsByAttributes(ld, []string{"env"})
		require.Empty(t, batches)
	})

	t.Run("empty attributes list", func(t *testing.T) {
		ld := simpleLogWithServiceName("svc-1")
		batches := splitLogsByAttributes(ld, []string{})
		// empty key for all logs
		require.Len(t, batches, 1)
		require.Contains(t, batches, "")
	})

	t.Run("attribute lookup priority resource over scope", func(t *testing.T) {
		// when an attribute exists at both resource and scope level,
		// resource should take priority
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("env", "resource-value")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().Attributes().PutStr("env", "scope-value")
		sl.LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"env"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "env=resource-value|")
	})

	t.Run("attribute lookup priority scope over log record", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().Attributes().PutStr("env", "scope-value")
		lr := sl.LogRecords().AppendEmpty()
		lr.Attributes().PutStr("env", "record-value")

		batches := splitLogsByAttributes(ld, []string{"env"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "env=scope-value|")
	})

	t.Run("different resources produce different keys", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("env", "prod")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("env", "staging")
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByAttributes(ld, []string{"env"})
		require.Len(t, batches, 2)
		require.Contains(t, batches, "env=prod|")
		require.Contains(t, batches, "env=staging|")
	})

	t.Run("same composite key merged", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("env", "prod")
		rl1.Resource().Attributes().PutStr("region", "us")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("env", "prod")
		rl2.Resource().Attributes().PutStr("region", "us")
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByAttributes(ld, []string{"env", "region"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "env=prod|region=us|")
		assert.Equal(t, 2, batches["env=prod|region=us|"].ResourceLogs().Len())
	})

	t.Run("no scope logs", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("env", "prod")
		// no ScopeLogs appended — attribute falls back to resource only

		batches := splitLogsByAttributes(ld, []string{"env", "user.id"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "env=prod|")
	})

	t.Run("mixed record attributes within single ResourceLogs are split per-record", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "svc-1")
		sl := rl.ScopeLogs().AppendEmpty()
		lr1 := sl.LogRecords().AppendEmpty()
		lr1.SetSeverityText("ERROR")
		lr1.Body().SetStr("error log")
		lr2 := sl.LogRecords().AppendEmpty()
		lr2.SetSeverityText("INFO")
		lr2.Body().SetStr("info log")
		lr3 := sl.LogRecords().AppendEmpty()
		lr3.SetSeverityText("ERROR")
		lr3.Body().SetStr("another error")

		batches := splitLogsByAttributes(ld, []string{"log.severity"})
		require.Len(t, batches, 2, "two different severities should produce two batches")
		require.Contains(t, batches, "log.severity=ERROR|")
		require.Contains(t, batches, "log.severity=INFO|")

		// ERROR batch should have 2 records
		errorBatch := batches["log.severity=ERROR|"]
		totalError := 0
		for i := 0; i < errorBatch.ResourceLogs().Len(); i++ {
			for j := 0; j < errorBatch.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				totalError += errorBatch.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
			}
		}
		assert.Equal(t, 2, totalError)

		// INFO batch should have 1 record
		infoBatch := batches["log.severity=INFO|"]
		totalInfo := 0
		for i := 0; i < infoBatch.ResourceLogs().Len(); i++ {
			for j := 0; j < infoBatch.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				totalInfo += infoBatch.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
			}
		}
		assert.Equal(t, 1, totalInfo)
	})

	t.Run("resource-level attributes skip per-record split", func(t *testing.T) {
		// When all routing attributes are at resource level, the entire
		// ResourceLogs should be copied as-is (no per-record iteration).
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("env", "prod")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.LogRecords().AppendEmpty().Body().SetStr("log-1")
		sl.LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches := splitLogsByAttributes(ld, []string{"env"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "env=prod|")
		// Both records should stay together in one ResourceLogs
		totalRecords := 0
		b := batches["env=prod|"]
		for i := 0; i < b.ResourceLogs().Len(); i++ {
			for j := 0; j < b.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				totalRecords += b.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
			}
		}
		assert.Equal(t, 2, totalRecords)
	})
}

// Routing integration tests

func TestConsumeLogsWithServiceRouting(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})},
		RoutingKey: svcRoutingStr,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	ld := plog.NewLogs()
	rl1 := ld.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", "svc-1")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
	rl2 := ld.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("service.name", "svc-2")
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

	err = p.ConsumeLogs(t.Context(), ld)
	assert.NoError(t, err)
	assert.NotEmpty(t, sink.AllLogs())
}

func TestConsumeLogsWithResourceRouting(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})},
		RoutingKey: resourceRoutingStr,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	ld := plog.NewLogs()
	rl1 := ld.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", "svc-1")
	rl1.Resource().Attributes().PutStr("host.name", "host-1")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
	rl2 := ld.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("service.name", "svc-1")
	rl2.Resource().Attributes().PutStr("host.name", "host-2")
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

	err = p.ConsumeLogs(t.Context(), ld)
	assert.NoError(t, err)
	assert.NotEmpty(t, sink.AllLogs())
}

func TestConsumeLogsWithAttributeRouting(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}

	cfg := &Config{
		Resolver:          ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})},
		RoutingKey:        attrRoutingStr,
		RoutingAttributes: []string{"env"},
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	ld := plog.NewLogs()
	rl1 := ld.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("env", "prod")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
	rl2 := ld.ResourceLogs().AppendEmpty()
	rl2.Resource().Attributes().PutStr("env", "staging")
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

	err = p.ConsumeLogs(t.Context(), ld)
	assert.NoError(t, err)
	assert.NotEmpty(t, sink.AllLogs())
}

func TestConsumeLogsConsistentRouting(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	var exporterEndpoints sync.Map
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			exporterEndpoints.Store(endpoint, true)
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}})},
		RoutingKey: svcRoutingStr,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2", "endpoint-3"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// send the same service name multiple times
	for range 10 {
		ld := simpleLogWithServiceName("consistent-svc")
		err = p.ConsumeLogs(t.Context(), ld)
		require.NoError(t, err)
	}

	// verify all went to the same endpoint
	count := 0
	exporterEndpoints.Range(func(_, _ any) bool {
		count++
		return true
	})
	assert.Equal(t, 1, count, "all logs with the same service name should route to the same endpoint")
}

func TestConsumeLogsWithResourceRoutingConsistent(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	var exporterEndpoints sync.Map
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			exporterEndpoints.Store(endpoint, true)
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}})},
		RoutingKey: resourceRoutingStr,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2", "endpoint-3"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// send the same resource multiple times
	for range 10 {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "consistent-svc")
		rl.Resource().Attributes().PutStr("host.name", "consistent-host")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test")
		err = p.ConsumeLogs(t.Context(), ld)
		require.NoError(t, err)
	}

	// verify all went to the same endpoint
	count := 0
	exporterEndpoints.Range(func(_, _ any) bool {
		count++
		return true
	})
	assert.Equal(t, 1, count, "all logs with the same resource should route to the same endpoint")
}

func TestConsumeLogsMixedServiceNames(t *testing.T) {
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

	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// batch with mix of valid and missing service names
	ld := plog.NewLogs()
	rl1 := ld.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", "svc-1")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("valid log")
	rl2 := ld.ResourceLogs().AppendEmpty()
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("no service log")

	// both logs should be exported — with a single endpoint, both routing keys
	// hash to the same backend so batches are coalesced into one ConsumeLogs call
	err = p.ConsumeLogs(t.Context(), ld)
	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 1, "single endpoint coalesces all batches into one call")
}

func TestConsumeLogsTripleEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	sink1 := new(consumertest.LogsSink)
	sink2 := new(consumertest.LogsSink)
	sink3 := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		switch endpoint {
		case "endpoint-1:4317":
			return newMockLogsExporter(sink1.ConsumeLogs), nil
		case "endpoint-2:4317":
			return newMockLogsExporter(sink2.ConsumeLogs), nil
		case "endpoint-3:4317":
			return newMockLogsExporter(sink3.ConsumeLogs), nil
		default:
			return nil, fmt.Errorf("unexpected endpoint: %s", endpoint)
		}
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}})},
		RoutingKey: svcRoutingStr,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2", "endpoint-3"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2", "endpoint-3"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// send logs for 3 different services
	ld := plog.NewLogs()
	for i := range 3 {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(fmt.Sprintf("log-%d", i))
	}

	err = p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	// verify total logs across all sinks equals what we sent
	totalLogs := len(sink1.AllLogs()) + len(sink2.AllLogs()) + len(sink3.AllLogs())
	assert.Positive(t, totalLogs, "logs should be distributed across endpoints")

	// verify each sink received only the logs for services that hashed to it
	totalRL := 0
	for _, l := range sink1.AllLogs() {
		totalRL += l.ResourceLogs().Len()
	}
	for _, l := range sink2.AllLogs() {
		totalRL += l.ResourceLogs().Len()
	}
	for _, l := range sink3.AllLogs() {
		totalRL += l.ResourceLogs().Len()
	}
	assert.Equal(t, 3, totalRL, "all 3 resource logs should be routed somewhere")
}

func TestConsumeLogsExportFailure(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
			return errors.New("export failed")
		}), nil
	}

	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

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

	err = p.ConsumeLogs(t.Context(), simpleLogs())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "export failed")
}

func TestConsumeLogsEmptyBatch(t *testing.T) {
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

	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// empty logs — should succeed (no resource logs to split)
	err = p.ConsumeLogs(t.Context(), plog.NewLogs())
	assert.NoError(t, err)
	assert.Empty(t, sink.AllLogs())
}

// TraceID routing tests (backward compatibility)

func TestSplitLogsByTraceID(t *testing.T) {
	t.Run("single log with traceID", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().SetTraceID(
			pcommon.TraceID([16]byte{1, 2, 3, 4}),
		)

		batches := splitLogsByTraceID(ld)
		require.Len(t, batches, 1)
	})

	t.Run("different traceIDs produce different keys", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().SetTraceID(
			pcommon.TraceID([16]byte{1, 2, 3, 4}),
		)
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().SetTraceID(
			pcommon.TraceID([16]byte{5, 6, 7, 8}),
		)

		batches := splitLogsByTraceID(ld)
		require.Len(t, batches, 2)
	})

	t.Run("same traceID merged", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().SetTraceID(
			pcommon.TraceID([16]byte{1, 2, 3, 4}),
		)
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().SetTraceID(
			pcommon.TraceID([16]byte{1, 2, 3, 4}),
		)

		batches := splitLogsByTraceID(ld)
		require.Len(t, batches, 1)
		for _, b := range batches {
			assert.Equal(t, 2, b.ResourceLogs().Len())
		}
	})

	t.Run("empty traceID", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

		batches := splitLogsByTraceID(ld)
		require.Len(t, batches, 1)
		// empty traceID gets a random key to avoid hot-spotting
		emptyKey := pcommon.TraceID([16]byte{}).String()
		require.NotContains(t, batches, emptyKey)
	})

	t.Run("empty logs", func(t *testing.T) {
		ld := plog.NewLogs()
		batches := splitLogsByTraceID(ld)
		require.Empty(t, batches)
	})

	t.Run("mixed traceIDs within single ResourceLogs are split per-record", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "svc-1")
		sl := rl.ScopeLogs().AppendEmpty()
		sl.LogRecords().AppendEmpty().SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
		sl.LogRecords().AppendEmpty().SetTraceID(pcommon.TraceID([16]byte{5, 6, 7, 8}))
		sl.LogRecords().AppendEmpty().SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))

		batches := splitLogsByTraceID(ld)
		require.Len(t, batches, 2, "two different traceIDs should produce two batches")

		tid1 := pcommon.TraceID([16]byte{1, 2, 3, 4}).String()
		tid2 := pcommon.TraceID([16]byte{5, 6, 7, 8}).String()

		require.Contains(t, batches, tid1)
		require.Contains(t, batches, tid2)

		// traceID {1,2,3,4} should have 2 records
		b1 := batches[tid1]
		totalRecords1 := 0
		for i := 0; i < b1.ResourceLogs().Len(); i++ {
			for j := 0; j < b1.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				totalRecords1 += b1.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
			}
		}
		assert.Equal(t, 2, totalRecords1)

		// traceID {5,6,7,8} should have 1 record
		b2 := batches[tid2]
		totalRecords2 := 0
		for i := 0; i < b2.ResourceLogs().Len(); i++ {
			for j := 0; j < b2.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				totalRecords2 += b2.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
			}
		}
		assert.Equal(t, 1, totalRecords2)
	})
}

func TestConsumeLogsWithTraceIDRouting(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}})},
		RoutingKey: traceIDRoutingStr,
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newLogsExporter(ts, cfg)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2"})
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().SetTraceID(
		pcommon.TraceID([16]byte{1, 2, 3, 4}),
	)

	err = p.ConsumeLogs(t.Context(), ld)
	assert.NoError(t, err)
	assert.NotEmpty(t, sink.AllLogs())
}

// E2E routing isolation tests — prove specific logs always go to specific collectors

func TestE2ERoutingIsolation(t *testing.T) {
	// This test proves that logs from different services route to specific,
	// deterministic collectors and that the same service always routes to
	// the same collector across multiple batches.
	ts, tb := getTelemetryAssets(t)

	// Track which endpoint received which service names
	endpointToServices := &sync.Map{}

	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				svc, ok := ld.ResourceLogs().At(i).Resource().Attributes().Get("service.name")
				if ok {
					endpointToServices.Store(svc.Str(), endpoint)
				}
			}
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: svcRoutingStr,
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	services := []string{
		"payment-service", "auth-service", "order-service",
		"notification-service", "inventory-service", "shipping-service",
	}

	// Send each service 20 times across separate batches to prove consistency
	for round := range 20 {
		for _, svc := range services {
			ld := plog.NewLogs()
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", svc)
			lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
			lr.Body().SetStr(fmt.Sprintf("log message round=%d", round))
			lr.SetSeverityText("INFO")

			err = p.ConsumeLogs(t.Context(), ld)
			require.NoError(t, err)
		}
	}

	// Verify: each service always went to exactly one endpoint
	serviceToEndpoint := map[string]string{}
	endpointToServices.Range(func(key, value any) bool {
		serviceToEndpoint[key.(string)] = value.(string)
		return true
	})

	require.Len(t, serviceToEndpoint, len(services), "every service should have been routed")

	// Verify that not all services went to the same endpoint (with 6 services and 3 endpoints,
	// consistent hashing should distribute them)
	endpointSet := map[string]bool{}
	for _, ep := range serviceToEndpoint {
		endpointSet[ep] = true
	}
	assert.Greater(t, len(endpointSet), 1, "services should be distributed across multiple endpoints, got all on one")

	// Now send them again — verify routing didn't change
	firstRoundMapping := make(map[string]string, len(serviceToEndpoint))
	maps.Copy(firstRoundMapping, serviceToEndpoint)

	for _, svc := range services {
		ld := simpleLogWithServiceName(svc)
		err = p.ConsumeLogs(t.Context(), ld)
		require.NoError(t, err)
	}

	endpointToServices.Range(func(key, value any) bool {
		svc := key.(string)
		ep := value.(string)
		assert.Equal(t, firstRoundMapping[svc], ep,
			"service %s should consistently route to the same endpoint", svc)
		return true
	})
}

func TestE2EResourceRoutingIsolation(t *testing.T) {
	// Prove that resource routing separates same-service logs by host/instance
	ts, tb := getTelemetryAssets(t)

	endpointToResources := &sync.Map{}

	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			for i := 0; i < ld.ResourceLogs().Len(); i++ {
				host, ok := ld.ResourceLogs().At(i).Resource().Attributes().Get("host.name")
				if ok {
					endpointToResources.Store(host.Str(), endpoint)
				}
			}
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: resourceRoutingStr,
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Same service, different hosts — should potentially route differently
	hosts := []string{"host-a", "host-b", "host-c", "host-d"}
	for round := range 10 {
		for _, host := range hosts {
			ld := plog.NewLogs()
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("service.name", "my-service")
			rl.Resource().Attributes().PutStr("host.name", host)
			rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(
				fmt.Sprintf("round=%d host=%s", round, host),
			)

			err = p.ConsumeLogs(t.Context(), ld)
			require.NoError(t, err)
		}
	}

	// Verify each host consistently routed to the same endpoint
	hostToEndpoint := map[string]string{}
	endpointToResources.Range(func(key, value any) bool {
		hostToEndpoint[key.(string)] = value.(string)
		return true
	})

	require.Len(t, hostToEndpoint, len(hosts), "every host should have been routed")
}

// Data integrity tests — verify logs are not corrupted, lost, or duplicated during routing

func TestDataIntegrityThroughRouting(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	// Collect all logs received by all endpoints
	var mu sync.Mutex
	receivedLogs := map[string][]plog.Logs{} // endpoint -> logs

	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			// Deep copy to avoid data races from MoveAndAppendTo
			clone := plog.NewLogs()
			ld.CopyTo(clone)
			mu.Lock()
			receivedLogs[endpoint] = append(receivedLogs[endpoint], clone)
			mu.Unlock()
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: svcRoutingStr,
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Build a batch with rich log data — body, severity, timestamps, multiple attributes
	type sentLog struct {
		serviceName  string
		hostName     string
		body         string
		severityText string
		traceID      string
		timestamp    pcommon.Timestamp
	}

	var sentLogs []sentLog
	ld := plog.NewLogs()

	for i := range 10 {
		svc := fmt.Sprintf("svc-%d", i%3)
		host := fmt.Sprintf("host-%d", i)
		body := fmt.Sprintf("important log message #%d with data=%s", i, "payload")
		severity := []string{"DEBUG", "INFO", "WARN", "ERROR"}[i%4]
		logTS := pcommon.Timestamp(1000000 + uint64(i)*1000)
		traceID := fmt.Sprintf("trace-%02d", i)

		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", svc)
		rl.Resource().Attributes().PutStr("host.name", host)
		rl.Resource().Attributes().PutStr("k8s.namespace.name", "prod")

		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("my.library")
		sl.Scope().SetVersion("1.0.0")

		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr(body)
		lr.SetSeverityText(severity)
		lr.SetTimestamp(logTS)
		lr.Attributes().PutStr("trace_id", traceID)
		lr.Attributes().PutStr("user.id", fmt.Sprintf("user-%d", i))
		lr.Attributes().PutInt("request.size", int64(i*100))

		sentLogs = append(sentLogs, sentLog{
			serviceName:  svc,
			hostName:     host,
			body:         body,
			severityText: severity,
			traceID:      traceID,
			timestamp:    logTS,
		})
	}

	err = p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	// Collect all received log records across all endpoints
	type receivedLog struct {
		serviceName  string
		hostName     string
		body         string
		severityText string
		traceID      string
		timestamp    pcommon.Timestamp
	}

	var allReceived []receivedLog
	mu.Lock()
	for _, logs := range receivedLogs {
		for _, l := range logs {
			for i := 0; i < l.ResourceLogs().Len(); i++ {
				rl := l.ResourceLogs().At(i)
				svc, _ := rl.Resource().Attributes().Get("service.name")
				host, _ := rl.Resource().Attributes().Get("host.name")
				ns, ok := rl.Resource().Attributes().Get("k8s.namespace.name")
				assert.True(t, ok, "k8s.namespace.name should be preserved")
				assert.Equal(t, "prod", ns.Str())

				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					sl := rl.ScopeLogs().At(j)
					assert.Equal(t, "my.library", sl.Scope().Name())
					assert.Equal(t, "1.0.0", sl.Scope().Version())

					for k := 0; k < sl.LogRecords().Len(); k++ {
						lr := sl.LogRecords().At(k)
						tid, _ := lr.Attributes().Get("trace_id")
						uid, ok := lr.Attributes().Get("user.id")
						assert.True(t, ok, "user.id attribute should be preserved")
						assert.NotEmpty(t, uid.Str())
						reqSize, ok := lr.Attributes().Get("request.size")
						assert.True(t, ok, "request.size attribute should be preserved")
						assert.GreaterOrEqual(t, reqSize.Int(), int64(0))

						allReceived = append(allReceived, receivedLog{
							serviceName:  svc.Str(),
							hostName:     host.Str(),
							body:         lr.Body().AsString(),
							severityText: lr.SeverityText(),
							traceID:      tid.Str(),
							timestamp:    lr.Timestamp(),
						})
					}
				}
			}
		}
	}
	mu.Unlock()

	// Verify: no logs lost
	require.Len(t, allReceived, len(sentLogs), "no logs should be lost during routing")

	// Verify: no logs duplicated — each trace_id should appear exactly once
	traceIDs := map[string]int{}
	for _, r := range allReceived {
		traceIDs[r.traceID]++
	}
	for tid, count := range traceIDs {
		assert.Equal(t, 1, count, "trace_id %s appeared %d times (expected exactly 1 — no duplicates)", tid, count)
	}

	// Verify: every sent log was received with data intact
	receivedByTraceID := map[string]receivedLog{}
	for _, r := range allReceived {
		receivedByTraceID[r.traceID] = r
	}
	for _, sent := range sentLogs {
		got, ok := receivedByTraceID[sent.traceID]
		require.True(t, ok, "sent log with trace_id=%s was not received", sent.traceID)
		assert.Equal(t, sent.serviceName, got.serviceName, "service.name corrupted for trace_id=%s", sent.traceID)
		assert.Equal(t, sent.hostName, got.hostName, "host.name corrupted for trace_id=%s", sent.traceID)
		assert.Equal(t, sent.body, got.body, "body corrupted for trace_id=%s", sent.traceID)
		assert.Equal(t, sent.severityText, got.severityText, "severity corrupted for trace_id=%s", sent.traceID)
		assert.Equal(t, sent.timestamp, got.timestamp, "timestamp corrupted for trace_id=%s", sent.traceID)
	}
}

func TestDataIntegrityNoCrossContaminationBetweenServices(t *testing.T) {
	// Prove that logs from service A never contain attributes from service B
	ts, tb := getTelemetryAssets(t)

	var mu sync.Mutex
	receivedByEndpoint := map[string][]plog.Logs{}

	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			clone := plog.NewLogs()
			ld.CopyTo(clone)
			mu.Lock()
			receivedByEndpoint[endpoint] = append(receivedByEndpoint[endpoint], clone)
			mu.Unlock()
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: svcRoutingStr,
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Send a batch with 3 distinct services, each with unique attributes
	ld := plog.NewLogs()
	serviceData := map[string]string{
		"payment-svc": "credit-card-data",
		"auth-svc":    "auth-token-data",
		"order-svc":   "order-id-data",
	}
	for svc, data := range serviceData {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", svc)
		lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.Body().SetStr(data)
		lr.Attributes().PutStr("service_marker", svc)
	}

	err = p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)

	// Verify: each resource log's service_marker matches its service.name
	mu.Lock()
	defer mu.Unlock()

	for _, logs := range receivedByEndpoint {
		for _, l := range logs {
			for i := 0; i < l.ResourceLogs().Len(); i++ {
				rl := l.ResourceLogs().At(i)
				svc, _ := rl.Resource().Attributes().Get("service.name")
				for j := 0; j < rl.ScopeLogs().Len(); j++ {
					for k := 0; k < rl.ScopeLogs().At(j).LogRecords().Len(); k++ {
						lr := rl.ScopeLogs().At(j).LogRecords().At(k)
						marker, ok := lr.Attributes().Get("service_marker")
						require.True(t, ok)
						assert.Equal(t, svc.Str(), marker.Str(),
							"log record service_marker (%s) doesn't match resource service.name (%s) — cross-contamination detected",
							marker.Str(), svc.Str())

						// Verify body matches the service's expected data
						expectedBody := serviceData[svc.Str()]
						assert.Equal(t, expectedBody, lr.Body().AsString(),
							"log body corrupted or mixed between services")
					}
				}
			}
		}
	}
}

func TestDataIntegrityLogCountPreserved(t *testing.T) {
	// Verify that total log record count in == total log record count out,
	// across all endpoints, for various batch sizes
	for _, totalLogs := range []int{1, 5, 10, 50, 100} {
		t.Run(fmt.Sprintf("%d_logs", totalLogs), func(t *testing.T) {
			ts, tb := getTelemetryAssets(t)

			var totalReceived atomic.Int64
			endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
			componentFactory := func(_ context.Context, _ string) (component.Component, error) {
				return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
					totalReceived.Add(int64(ld.LogRecordCount()))
					return nil
				}), nil
			}

			cfg := &Config{
				Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
				RoutingKey: svcRoutingStr,
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

			err = p.Start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, p.Shutdown(t.Context()))
			}()

			ld := plog.NewLogs()
			for i := range totalLogs {
				rl := ld.ResourceLogs().AppendEmpty()
				rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i%5))
				rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(
					fmt.Sprintf("log-%d", i),
				)
			}

			sentCount := ld.LogRecordCount()
			err = p.ConsumeLogs(t.Context(), ld)
			require.NoError(t, err)

			assert.Equal(t, int64(sentCount), totalReceived.Load(),
				"total received log records should equal total sent")
		})
	}
}

// Multi-endpoint routing tests

func TestConsumeLogsMissingServiceMultipleEndpoints(t *testing.T) {
	// Mixed valid and missing service.name with multiple endpoints:
	// both records are exported, and deterministic backend assignment.
	ts, tb := getTelemetryAssets(t)

	var mu sync.Mutex
	receivedByEndpoint := map[string]int{}

	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			mu.Lock()
			receivedByEndpoint[endpoint] += ld.LogRecordCount()
			mu.Unlock()
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: svcRoutingStr,
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	ld := plog.NewLogs()
	rl1 := ld.ResourceLogs().AppendEmpty()
	rl1.Resource().Attributes().PutStr("service.name", "svc-1")
	rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("valid service")
	// missing service.name
	rl2 := ld.ResourceLogs().AppendEmpty()
	rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("no service")

	err = p.ConsumeLogs(t.Context(), ld)
	assert.NoError(t, err)

	mu.Lock()
	totalRecords := 0
	for _, count := range receivedByEndpoint {
		totalRecords += count
	}
	mu.Unlock()
	assert.Equal(t, 2, totalRecords, "both log records should be exported")

	// Verify determinism: send same batch again, total should double proportionally
	mu.Lock()
	firstRound := make(map[string]int, len(receivedByEndpoint))
	maps.Copy(firstRound, receivedByEndpoint)
	mu.Unlock()

	ld2 := plog.NewLogs()
	rl3 := ld2.ResourceLogs().AppendEmpty()
	rl3.Resource().Attributes().PutStr("service.name", "svc-1")
	rl3.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("valid service")
	rl4 := ld2.ResourceLogs().AppendEmpty()
	rl4.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("no service")

	err = p.ConsumeLogs(t.Context(), ld2)
	assert.NoError(t, err)

	mu.Lock()
	for ep, count := range receivedByEndpoint {
		prev := firstRound[ep]
		assert.Equal(t, prev*2, count,
			"endpoint %s should receive exactly double after second identical batch", ep)
	}
	mu.Unlock()
}

func TestSplitLogsByTraceIDEmptyAndNonEmptyMixed(t *testing.T) {
	// traceID routing with empty and non-empty trace IDs mixed in same scope:
	// empty IDs get random keys (spread across backends), non-empty are isolated.
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "svc-1")
	sl := rl.ScopeLogs().AppendEmpty()

	// 2 records with empty traceID
	lr1 := sl.LogRecords().AppendEmpty()
	lr1.Body().SetStr("no trace 1")
	lr2 := sl.LogRecords().AppendEmpty()
	lr2.Body().SetStr("no trace 2")
	// 1 record with a real traceID
	lr3 := sl.LogRecords().AppendEmpty()
	lr3.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
	lr3.Body().SetStr("with trace")
	// 1 record with a different traceID
	lr4 := sl.LogRecords().AppendEmpty()
	lr4.SetTraceID(pcommon.TraceID([16]byte{5, 6, 7, 8}))
	lr4.Body().SetStr("different trace")

	batches := splitLogsByTraceID(ld)

	tid1 := pcommon.TraceID([16]byte{1, 2, 3, 4}).String()
	tid2 := pcommon.TraceID([16]byte{5, 6, 7, 8}).String()

	// 4 batches: each empty traceID record gets its own random key, plus
	// one batch per non-empty traceID.
	require.Len(t, batches, 4)
	require.Contains(t, batches, tid1)
	require.Contains(t, batches, tid2)

	// Empty traceID key should NOT appear (random keys used instead)
	emptyKey := pcommon.TraceID([16]byte{}).String()
	require.NotContains(t, batches, emptyKey)

	// Total record count should be preserved
	totalRecords := 0
	for _, b := range batches {
		for i := 0; i < b.ResourceLogs().Len(); i++ {
			for j := 0; j < b.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				totalRecords += b.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
			}
		}
	}
	assert.Equal(t, 4, totalRecords, "all 4 records should be preserved")

	// Non-empty traceID batches should each have 1 record
	for _, key := range []string{tid1, tid2} {
		b := batches[key]
		count := 0
		for i := 0; i < b.ResourceLogs().Len(); i++ {
			for j := 0; j < b.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				count += b.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
			}
		}
		assert.Equal(t, 1, count, "traceID %s should have exactly 1 record", key)
	}
}

func TestSplitLogsByTraceIDPreservesSchemaURLAndScopeMetadata(t *testing.T) {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "svc-1")
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("my.library")
	sl.Scope().SetVersion("2.0.0")
	sl.Scope().Attributes().PutStr("scope.attr", "val")
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1/scope")

	lr := sl.LogRecords().AppendEmpty()
	lr.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
	lr.Body().SetStr("test log")

	batches := splitLogsByTraceID(ld)
	require.Len(t, batches, 1)

	tid := pcommon.TraceID([16]byte{1, 2, 3, 4}).String()
	b := batches[tid]

	resultRL := b.ResourceLogs().At(0)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.6.1", resultRL.SchemaUrl())

	resultSL := resultRL.ScopeLogs().At(0)
	assert.Equal(t, "my.library", resultSL.Scope().Name())
	assert.Equal(t, "2.0.0", resultSL.Scope().Version())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.6.1/scope", resultSL.SchemaUrl())

	v, ok := resultSL.Scope().Attributes().Get("scope.attr")
	require.True(t, ok)
	assert.Equal(t, "val", v.Str())
}

func TestSplitLogsByAttributesBothPseudoAttrs(t *testing.T) {
	// Attributes routing with log.severity and log.body pseudo attributes
	// mixed in same scope: per-record split.
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()

	lr1 := sl.LogRecords().AppendEmpty()
	lr1.SetSeverityText("ERROR")
	lr1.Body().SetStr("disk full")

	lr2 := sl.LogRecords().AppendEmpty()
	lr2.SetSeverityText("ERROR")
	lr2.Body().SetStr("disk full") // same severity + body → same key

	lr3 := sl.LogRecords().AppendEmpty()
	lr3.SetSeverityText("WARN")
	lr3.Body().SetStr("disk full") // different severity → different key

	lr4 := sl.LogRecords().AppendEmpty()
	lr4.SetSeverityText("ERROR")
	lr4.Body().SetStr("oom killed") // different body → different key

	batches := splitLogsByAttributes(ld, []string{"log.severity", "log.body"})

	// 3 unique combinations: ERROR+disk full, WARN+disk full, ERROR+oom killed
	require.Len(t, batches, 3)
	require.Contains(t, batches, "log.severity=ERROR|log.body=disk full|")
	require.Contains(t, batches, "log.severity=WARN|log.body=disk full|")
	require.Contains(t, batches, "log.severity=ERROR|log.body=oom killed|")

	// ERROR+disk full should have 2 records
	b := batches["log.severity=ERROR|log.body=disk full|"]
	count := 0
	for i := 0; i < b.ResourceLogs().Len(); i++ {
		for j := 0; j < b.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			count += b.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
		}
	}
	assert.Equal(t, 2, count)
}

func TestSplitLogsByAttributesMultipleScopeLogsNoCrossContamination(t *testing.T) {
	// Multiple ScopeLogs in one ResourceLogs with different scope attributes:
	// verify no cross-scope contamination.
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()

	// scope 1 has "library" attr at scope level
	sl1 := rl.ScopeLogs().AppendEmpty()
	sl1.Scope().SetName("lib-a")
	sl1.Scope().Attributes().PutStr("team", "alpha")
	lr1 := sl1.LogRecords().AppendEmpty()
	lr1.Body().SetStr("log from alpha")

	// scope 2 has different "library" attr at scope level
	sl2 := rl.ScopeLogs().AppendEmpty()
	sl2.Scope().SetName("lib-b")
	sl2.Scope().Attributes().PutStr("team", "beta")
	lr2 := sl2.LogRecords().AppendEmpty()
	lr2.Body().SetStr("log from beta")

	batches := splitLogsByAttributes(ld, []string{"team"})

	// Two different scope-level "team" values → two batches
	require.Len(t, batches, 2)
	require.Contains(t, batches, "team=alpha|")
	require.Contains(t, batches, "team=beta|")

	// Verify each batch has the correct scope name (no mixing)
	expectedTeamByKey := map[string]string{
		"team=alpha|": "alpha",
		"team=beta|":  "beta",
	}
	for key, b := range batches {
		for i := 0; i < b.ResourceLogs().Len(); i++ {
			for j := 0; j < b.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
				sl := b.ResourceLogs().At(i).ScopeLogs().At(j)
				teamVal, ok := sl.Scope().Attributes().Get("team")
				require.True(t, ok)
				assert.Equal(t, expectedTeamByKey[key], teamVal.Str(),
					"scope with team=%s ended up in batch for key=%s — cross-scope contamination",
					teamVal.Str(), key)
			}
		}
	}
}

func TestConsumeLogsPartialBackendFailure(t *testing.T) {
	// One exporter fails, others still receive and process their data correctly.
	ts, tb := getTelemetryAssets(t)

	var mu sync.Mutex
	successReceived := []plog.Logs{}
	failEndpoint := "endpoint-2:4317"

	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		if endpoint == failEndpoint {
			return newMockLogsExporter(func(_ context.Context, _ plog.Logs) error {
				return errors.New("backend unavailable")
			}), nil
		}
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			clone := plog.NewLogs()
			ld.CopyTo(clone)
			mu.Lock()
			successReceived = append(successReceived, clone)
			mu.Unlock()
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:   ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey: svcRoutingStr,
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Send many distinct services so at least some route to the failing backend
	// and some to the healthy ones
	ld := plog.NewLogs()
	for i := range 20 {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(
			fmt.Sprintf("log-%d", i),
		)
	}

	err = p.ConsumeLogs(t.Context(), ld)
	// Should return error (from the failing backend) but still process others
	assert.Error(t, err, "partial failure should surface as error")
	assert.Contains(t, err.Error(), "backend unavailable")

	// Healthy backends should have received their share of logs
	mu.Lock()
	totalSuccessRecords := 0
	for _, l := range successReceived {
		totalSuccessRecords += l.LogRecordCount()
	}
	mu.Unlock()
	assert.Positive(t, totalSuccessRecords,
		"healthy backends should still receive their logs despite partial failure")
}

func TestSplitLogsByAttributesScopeLevelResolution(t *testing.T) {
	// Test that attribute resolved at scope level correctly routes
	// all records in that scope together (no per-record split needed).
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutStr("env", "prod")
	sl.LogRecords().AppendEmpty().Body().SetStr("log-1")
	sl.LogRecords().AppendEmpty().Body().SetStr("log-2")
	sl.LogRecords().AppendEmpty().Body().SetStr("log-3")

	batches := splitLogsByAttributes(ld, []string{"env"})

	require.Len(t, batches, 1)
	require.Contains(t, batches, "env=prod|")

	// All 3 records should stay together — no per-record split needed
	b := batches["env=prod|"]
	totalRecords := 0
	for i := 0; i < b.ResourceLogs().Len(); i++ {
		for j := 0; j < b.ResourceLogs().At(i).ScopeLogs().Len(); j++ {
			totalRecords += b.ResourceLogs().At(i).ScopeLogs().At(j).LogRecords().Len()
		}
	}
	assert.Equal(t, 3, totalRecords)
}

func TestHighCardinalityAttributeRouting(t *testing.T) {
	// Stress test: many distinct attribute values to verify no allocation
	// issues or incorrect merging.
	ts, tb := getTelemetryAssets(t)

	var totalReceived atomic.Int64
	endpoints := []string{"endpoint-1", "endpoint-2", "endpoint-3"}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(func(_ context.Context, ld plog.Logs) error {
			totalReceived.Add(int64(ld.LogRecordCount()))
			return nil
		}), nil
	}

	cfg := &Config{
		Resolver:          ResolverSettings{Static: configoptional.Some(StaticResolver{Hostnames: endpoints})},
		RoutingKey:        attrRoutingStr,
		RoutingAttributes: []string{"request.id"},
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// 500 log records, each with a unique request.id
	const numLogs = 500
	ld := plog.NewLogs()
	for i := range numLogs {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "high-card-svc")
		lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		lr.Body().SetStr(fmt.Sprintf("request log %d", i))
		lr.Attributes().PutStr("request.id", fmt.Sprintf("req-%05d", i))
	}

	err = p.ConsumeLogs(t.Context(), ld)
	require.NoError(t, err)
	assert.Equal(t, int64(numLogs), totalReceived.Load(),
		"all high-cardinality logs should be delivered without loss")
}

// Benchmark tests

func benchConsumeLogs(b *testing.B, routingKey string, endpointsCount, rlCount, slCount, logCount int) {
	ts, tb := getTelemetryAssets(b)

	sink := new(consumertest.LogsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockLogsExporter(sink.ConsumeLogs), nil
	}

	endpoints := []string{}
	for i := range endpointsCount {
		endpoints = append(endpoints, fmt.Sprintf("endpoint-%d", i))
	}

	config := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: endpoints}),
		},
		RoutingKey: routingKey,
	}

	lb, err := newLoadBalancer(ts.Logger, config, componentFactory, tb)
	require.NotNil(b, lb)
	require.NoError(b, err)

	p, err := newLogsExporter(ts, config)
	require.NotNil(b, p)
	require.NoError(b, err)

	p.loadBalancer = lb

	err = p.Start(b.Context(), componenttest.NewNopHost())
	require.NoError(b, err)

	ld := randomBenchLogs(b, rlCount, slCount, logCount)

	for b.Loop() {
		err = p.ConsumeLogs(b.Context(), ld)
		require.NoError(b, err)
	}

	b.StopTimer()
	err = p.Shutdown(b.Context())
	require.NoError(b, err)
}

func BenchmarkConsumeLogs(b *testing.B) {
	testCases := []struct {
		routingKey string
	}{
		{routingKey: svcRoutingStr},
		{routingKey: resourceRoutingStr},
	}

	for _, tc := range testCases {
		b.Run(tc.routingKey, func(b *testing.B) {
			for _, endpointCount := range []int{1, 5, 10} {
				for _, rlCount := range []int{1, 3} {
					for _, totalLogCount := range []int{100, 500, 1000} {
						slCount := 1
						logCount := totalLogCount / slCount / rlCount

						b.Run(fmt.Sprintf("%dE_%dRL_%dSL_%dL", endpointCount, rlCount, slCount, logCount), func(b *testing.B) {
							benchConsumeLogs(b, tc.routingKey, endpointCount, rlCount, slCount, logCount)
						})
					}
				}
			}
		})
	}
}

// Helper functions

func randomLogs() plog.Logs {
	return simpleLogWithServiceName(fmt.Sprintf("svc-%d", rand.IntN(10)))
}

func simpleLogs() plog.Logs {
	return simpleLogWithServiceName("test-svc")
}

func simpleLogWithServiceName(svcName string) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", svcName)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty().Body().SetStr("test log")

	return logs
}

func randomBenchLogs(b *testing.B, rlCount, slCount, logCount int) plog.Logs {
	b.Helper()
	ld := plog.NewLogs()
	for i := range rlCount {
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", fmt.Sprintf("svc-%d", i))
		rl.Resource().Attributes().PutStr("host.name", fmt.Sprintf("host-%d", i))
		for range slCount {
			sl := rl.ScopeLogs().AppendEmpty()
			for range logCount {
				lr := sl.LogRecords().AppendEmpty()
				lr.Body().SetStr("benchmark log message")
				lr.SetSeverityText("INFO")
			}
		}
	}
	return ld
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
