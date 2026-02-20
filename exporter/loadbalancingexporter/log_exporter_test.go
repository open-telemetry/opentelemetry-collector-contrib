// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
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

	// test with a log that has no service.name — should return permanent error
	logWithoutSvc := plog.NewLogs()
	rl := logWithoutSvc.ResourceLogs().AppendEmpty()
	rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test")

	err = p.ConsumeLogs(t.Context(), logWithoutSvc)

	// verify — permanent error, no logs exported
	assert.Error(t, err)
	assert.Len(t, sink.AllLogs(), 0)
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
		batches, errs := splitLogsByServiceName(ld)
		require.Empty(t, errs)
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

		batches, errs := splitLogsByServiceName(ld)
		require.Empty(t, errs)
		require.Len(t, batches, 2)
		require.Contains(t, batches, "svc-1")
		require.Contains(t, batches, "svc-2")
	})

	t.Run("missing service name", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches, errs := splitLogsByServiceName(ld)
		require.Len(t, errs, 1)
		require.Len(t, batches, 0)
	})

	t.Run("duplicate service names merged", func(t *testing.T) {
		ld := plog.NewLogs()
		rl1 := ld.ResourceLogs().AppendEmpty()
		rl1.Resource().Attributes().PutStr("service.name", "svc-1")
		rl1.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
		rl2 := ld.ResourceLogs().AppendEmpty()
		rl2.Resource().Attributes().PutStr("service.name", "svc-1")
		rl2.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

		batches, errs := splitLogsByServiceName(ld)
		require.Empty(t, errs)
		require.Len(t, batches, 1)
		require.Contains(t, batches, "svc-1")
		assert.Equal(t, 2, batches["svc-1"].ResourceLogs().Len())
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
}

func TestSplitLogsByAttributes(t *testing.T) {
	t.Run("single attribute", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("env", "prod")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"env"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "prod")
	})

	t.Run("multiple attributes composite key", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("env", "prod")
		rl.Resource().Attributes().PutStr("region", "us-east")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"env", "region"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "produs-east")
	})

	t.Run("missing attribute uses empty string", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"nonexistent"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "")
	})

	t.Run("pseudo attribute log.severity", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.SetSeverityText("ERROR")

		batches := splitLogsByAttributes(ld, []string{"log.severity"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "ERROR")
	})

	t.Run("pseudo attribute log.body", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.Body().SetStr("my log body")

		batches := splitLogsByAttributes(ld, []string{"log.body"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "my log body")
	})

	t.Run("scope attribute", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().Attributes().PutStr("library", "mylib")
		sl.LogRecords().AppendEmpty().Body().SetStr("log-1")

		batches := splitLogsByAttributes(ld, []string{"library"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "mylib")
	})

	t.Run("log record attribute", func(t *testing.T) {
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		lr := sl.LogRecords().AppendEmpty()
		lr.Attributes().PutStr("user.id", "user-123")

		batches := splitLogsByAttributes(ld, []string{"user.id"})
		require.Len(t, batches, 1)
		require.Contains(t, batches, "user-123")
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

func simpleLogWithID(id pcommon.TraceID) plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("service.name", "test-svc")
	sl := rl.ScopeLogs().AppendEmpty()
	sl.LogRecords().AppendEmpty().SetTraceID(id)

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
