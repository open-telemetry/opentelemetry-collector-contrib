// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenthelper"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
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
			&Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
			},
			errNoResolver,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// test
			_, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), tt.config)

			// verify
			require.Equal(t, tt.err, err)
		})
	}
}

func TestLogExporterStart(t *testing.T) {
	for _, tt := range []struct {
		desc string
		le   *logExporterImp
		err  error
	}{
		{
			"ok",
			func() *logExporterImp {
				p, _ := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *logExporterImp {
				// prepare
				lb, _ := newLoadBalancer(componenttest.NewNopExporterCreateSettings(), simpleConfig(), nil)
				p, _ := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())

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
			defer p.Shutdown(context.Background())

			// verify
			require.Equal(t, tt.err, res)
		})
	}
}

func TestLogExporterShutdown(t *testing.T) {
	p, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestConsumeLogs(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Exporter, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(componenttest.NewNopExporterCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.exporters["endpoint-1"] = newNopMockLogsExporter()
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	// test
	res := p.ConsumeLogs(context.Background(), simpleLogs())

	// verify
	assert.Nil(t, res)
}

func TestConsumeLogsExporterNotFound(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Exporter, error) {
		return newNopMockTracesExporter(), nil
	}
	lb, err := newLoadBalancer(componenttest.NewNopExporterCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	// test
	res := p.ConsumeLogs(context.Background(), simpleLogs())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("couldn't find the exporter for the endpoint %q", "endpoint-1"))
}

func TestConsumeLogsUnexpectedExporterType(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Exporter, error) {
		return newNopMockExporter(), nil
	}
	lb, err := newLoadBalancer(componenttest.NewNopExporterCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.exporters["endpoint-1"] = newNopMockExporter()
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	// test
	res := p.ConsumeLogs(context.Background(), simpleLogs())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("unable to export logs, unexpected exporter type: expected *component.LogsExporter but got %T", newNopMockExporter()))
}

func TestLogBatchWithTwoTraces(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Exporter, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(componenttest.NewNopExporterCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	sink := new(consumertest.LogsSink)
	lb.exporters["endpoint-1"] = newMockLogsExporter(sink.ConsumeLogs)
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	first := simpleLogs()
	second := simpleLogWithID(pdata.NewTraceID([16]byte{2, 3, 4, 5}))
	batch := pdata.NewLogs()
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
		batch pdata.Logs
	}{
		{
			"no resource logs",
			pdata.NewLogs(),
		},
		{
			"no instrumentation library logs",
			func() pdata.Logs {
				batch := pdata.NewLogs()
				batch.ResourceLogs().AppendEmpty()
				return batch
			}(),
		},
		{
			"no logs",
			func() pdata.Logs {
				batch := pdata.NewLogs()
				batch.ResourceLogs().AppendEmpty().InstrumentationLibraryLogs().AppendEmpty()
				return batch
			}(),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res := traceIDFromLogs(tt.batch)
			assert.Equal(t, pdata.InvalidTraceID(), res)
		})
	}
}

func TestLogsWithoutTraceID(t *testing.T) {
	componentFactory := func(ctx context.Context, endpoint string) (component.Exporter, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(componenttest.NewNopExporterCreateSettings(), simpleConfig(), componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), simpleConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	sink := new(consumertest.LogsSink)
	lb.exporters["endpoint-1"] = newMockLogsExporter(sink.ConsumeLogs)
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	// test
	err = p.ConsumeLogs(context.Background(), simpleLogWithoutID())

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllLogs(), 1)
}

func TestRollingUpdatesWhenConsumeLogs(t *testing.T) {
	// this test is based on the discussion in the following issue for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	// prepare

	// simulate rolling updates, the dns resolver should resolve in the following order
	// ["127.0.0.1"] -> ["127.0.0.1", "127.0.0.2"] -> ["127.0.0.2"]
	res, err := newDNSResolver(zap.NewNop(), "service-1", "")
	require.NoError(t, err)

	resolverCh := make(chan struct{}, 1)
	counter := 0
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
				counter++
			}()

			if counter <= 2 {
				return resolve[counter], nil
			}

			if counter == 3 {
				// stop as soon as rolling updates end
				resolverCh <- struct{}{}
			}

			return resolve[2], nil
		},
	}
	res.resInterval = 10 * time.Millisecond

	cfg := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
		Resolver: ResolverSettings{
			DNS: &DNSResolver{Hostname: "service-1", Port: ""},
		},
	}
	componentFactory := func(ctx context.Context, endpoint string) (component.Exporter, error) {
		return newNopMockLogsExporter(), nil
	}
	lb, err := newLoadBalancer(componenttest.NewNopExporterCreateSettings(), cfg, componentFactory)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newLogsExporter(componenttest.NewNopExporterCreateSettings(), cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb

	var counter1, counter2 int64
	defaultExporters := map[string]component.Exporter{
		"127.0.0.1": newMockLogsExporter(func(ctx context.Context, ld pdata.Logs) error {
			atomic.AddInt64(&counter1, 1)
			// simulate an unreachable backend
			time.Sleep(10 * time.Second)
			return nil
		},
		),
		"127.0.0.2": newMockLogsExporter(func(ctx context.Context, ld pdata.Logs) error {
			atomic.AddInt64(&counter2, 1)
			return nil
		},
		),
	}

	// test
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())
	// ensure using default exporters
	lb.updateLock.Lock()
	lb.exporters = defaultExporters
	lb.updateLock.Unlock()
	lb.res.onChange(func(endpoints []string) {
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
				go p.ConsumeLogs(ctx, randomLogs())
			}
		}
	}(ctx)

	// give limited but enough time to rolling updates. otherwise this test
	// will still pass due to the 10 secs of sleep that is used to simulate
	// unreachable backends.
	go func() {
		time.Sleep(50 * time.Millisecond)
		resolverCh <- struct{}{}
	}()

	<-resolverCh
	cancel()
	<-consumeCh

	// verify
	require.Equal(t, []string{"127.0.0.2"}, res.endpoints)
	require.Greater(t, atomic.LoadInt64(&counter1), int64(0))
	require.Greater(t, atomic.LoadInt64(&counter2), int64(0))
}

func randomLogs() pdata.Logs {
	return simpleLogWithID(random())
}

func simpleLogs() pdata.Logs {
	return simpleLogWithID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
}

func simpleLogWithID(id pdata.TraceID) pdata.Logs {
	logs := pdata.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	ill.Logs().AppendEmpty().SetTraceID(id)

	return logs
}

func simpleLogWithoutID() pdata.Logs {
	logs := pdata.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	ill.Logs().AppendEmpty()

	return logs
}

type mockLogsExporter struct {
	component.Component
	ConsumeLogsFn func(ctx context.Context, ld pdata.Logs) error
}

func (e *mockLogsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockLogsExporter) ConsumeLogs(ctx context.Context, ld pdata.Logs) error {
	if e.ConsumeLogsFn == nil {
		return nil
	}
	return e.ConsumeLogsFn(ctx, ld)
}

func newMockLogsExporter(ConsumeLogsFn func(ctx context.Context, ld pdata.Logs) error) component.LogsExporter {
	return &mockLogsExporter{
		Component:     componenthelper.New(),
		ConsumeLogsFn: ConsumeLogsFn,
	}
}

func newNopMockLogsExporter() component.LogsExporter {
	return &mockLogsExporter{
		Component: componenthelper.New(),
		ConsumeLogsFn: func(ctx context.Context, ld pdata.Logs) error {
			return nil
		},
	}
}
