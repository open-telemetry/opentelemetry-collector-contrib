// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const (
	ilsName1          = "library-1"
	ilsName2          = "library-2"
	keyAttr1          = "resattr-1"
	keyAttr2          = "resattr-2"
	valueAttr1        = "resvaluek1"
	valueAttr2        = 10
	signal1Name       = "sig-1"
	signal2Name       = "sig-2"
	signal1Attr1Key   = "sigattr1k"
	signal1Attr1Value = "sigattr1v"
	signal1Attr2Key   = "sigattr2k"
	signal1Attr2Value = 20
	signal1Attr3Key   = "sigattr3k"
	signal1Attr3Value = true
	signal1Attr4Key   = "sigattr4k"
	signal1Attr4Value = 3.3
	serviceName1      = "service-name-1"
	serviceName2      = "service-name-2"
)

func TestNewMetricsExporter(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	for _, tt := range []struct {
		desc   string
		config *Config
		err    error
	}{
		{
			"empty routing key",
			&Config{},
			errNoResolver,
		},
		{
			"service",
			serviceBasedRoutingConfig(),
			nil,
		},
		{
			"metric",
			metricNameBasedRoutingConfig(),
			nil,
		},
		{
			"resource",
			resourceBasedRoutingConfig(),
			nil,
		},
		{
			"traceID",
			&Config{
				RoutingKey: traceIDRoutingStr,
			},
			errNoResolver,
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			// test
			_, err := newMetricsExporter(ts, tt.config)

			// verify
			require.Equal(t, tt.err, err)
		})
	}
}

func TestMetricsExporterStart(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	for _, tt := range []struct {
		desc string
		te   *metricExporterImp
		err  error
	}{
		{
			"ok",
			func() *metricExporterImp {
				p, _ := newMetricsExporter(ts, serviceBasedRoutingConfig())
				return p
			}(),
			nil,
		},
		{
			"error",
			func() *metricExporterImp {
				lb, err := newLoadBalancer(ts.Logger, serviceBasedRoutingConfig(), nil, tb)
				require.NoError(t, err)

				p, _ := newMetricsExporter(ts, serviceBasedRoutingConfig())

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
			p := tt.te

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

func TestMetricsExporterShutdown(t *testing.T) {
	ts, _ := getTelemetryAssets(t)
	p, err := newMetricsExporter(ts, serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.NoError(t, res)
}

// loadMetricsMap will parse the given yaml file into a map[string]pmetric.Metrics
func loadMetricsMap(t *testing.T, path string) map[string]pmetric.Metrics {
	b, err := os.ReadFile(path)
	require.NoError(t, err)

	var expectedOutputRaw map[string]any
	err = yaml.Unmarshal(b, &expectedOutputRaw)
	require.NoError(t, err)

	expectedOutput := map[string]pmetric.Metrics{}
	for key, data := range expectedOutputRaw {
		b, err = jsoniter.Marshal(data)
		require.NoError(t, err)

		unmarshaller := &pmetric.JSONUnmarshaler{}
		md, err := unmarshaller.UnmarshalMetrics(b)
		require.NoError(t, err)

		expectedOutput[key] = md
	}

	return expectedOutput
}

func compareMetricsMaps(t *testing.T, expected map[string]pmetric.Metrics, actual map[string]pmetric.Metrics) {
	expectedKeys := make([]string, 0, len(expected))
	for key := range expected {
		expectedKeys = append(expectedKeys, key)
	}

	actualKeys := make([]string, 0, len(actual))
	for key := range actual {
		actualKeys = append(actualKeys, key)
	}

	require.ElementsMatch(t, expectedKeys, actualKeys, "Maps have differing keys")

	for key, actualMD := range actual {
		expectedMD := expected[key]
		t.Logf("Comparing map values for key: %s", key)
		require.NoError(t, pmetrictest.CompareMetrics(
			expectedMD, actualMD,
			// We have to ignore ordering, because we do MergeMetrics() inside a map
			// iteration. And golang map iteration order is random. This means the
			// order of the merges is random
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreMetricsOrder(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
		))
	}
}

func TestSplitMetricsByResourceServiceName(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"basic_resource_service_name",
		"duplicate_resource_service_name",
	}

	for _, tc := range testCases {
		testName := tc

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join("testdata", "metrics", "split_metrics", testName)

			input, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
			require.NoError(t, err)

			expectedOutput := loadMetricsMap(t, filepath.Join(dir, "output.yaml"))

			output, err := splitMetricsByResourceServiceName(input)
			require.NoError(t, err)
			compareMetricsMaps(t, expectedOutput, output)
		})
	}
}

func TestSplitMetricsByResourceServiceNameFailsIfMissingServiceNameAttribute(t *testing.T) {
	t.Parallel()

	input, err := golden.ReadMetrics(filepath.Join("testdata", "metrics", "split_metrics", "missing_service_name", "input.yaml"))
	require.NoError(t, err)

	_, err = splitMetricsByResourceServiceName(input)
	require.Error(t, err)
}

func TestSplitMetrics(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		splitFunc func(md pmetric.Metrics) map[string]pmetric.Metrics
	}{
		{
			name:      "basic_resource_id",
			splitFunc: splitMetricsByResourceID,
		},
		{
			name:      "duplicate_resource_id",
			splitFunc: splitMetricsByResourceID,
		},
		{
			name:      "basic_metric_name",
			splitFunc: splitMetricsByMetricName,
		},
		{
			name:      "duplicate_metric_name",
			splitFunc: splitMetricsByMetricName,
		},
		{
			name:      "basic_stream_id",
			splitFunc: splitMetricsByStreamID,
		},
		{
			name:      "duplicate_stream_id",
			splitFunc: splitMetricsByStreamID,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join("testdata", "metrics", "split_metrics", tc.name)

			input, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
			require.NoError(t, err)

			expectedOutput := loadMetricsMap(t, filepath.Join(dir, "output.yaml"))

			output := tc.splitFunc(input)
			require.NoError(t, err)
			compareMetricsMaps(t, expectedOutput, output)
		})
	}
}

func TestConsumeMetrics_SingleEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	t.Parallel()

	testCases := []struct {
		name       string
		routingKey string
	}{
		{
			name:       "resource_service_name",
			routingKey: svcRoutingStr,
		},
		{
			name:       "resource_id",
			routingKey: resourceRoutingStr,
		},
		{
			name:       "metric_name",
			routingKey: metricNameRoutingStr,
		},
		{
			name:       "stream_id",
			routingKey: streamIDRoutingStr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			createSettings := ts
			config := &Config{
				Resolver: ResolverSettings{
					Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
				},
				RoutingKey: tc.routingKey,
			}

			p, err := newMetricsExporter(createSettings, config)
			require.NoError(t, err)
			require.NotNil(t, p)

			// newMetricsExporter will internally create a loadBalancer instance which is
			// hardcoded to use OTLP exporters
			// We manually override that to use our testing sink
			sink := consumertest.MetricsSink{}
			componentFactory := func(_ context.Context, _ string) (component.Component, error) {
				return newMockMetricsExporter(sink.ConsumeMetrics), nil
			}

			lb, err := newLoadBalancer(ts.Logger, config, componentFactory, tb)
			require.NoError(t, err)
			require.NotNil(t, lb)

			lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
			lb.res = &mockResolver{
				triggerCallbacks: true,
				onResolve: func(_ context.Context) ([]string, error) {
					return []string{"endpoint-1"}, nil
				},
			}
			p.loadBalancer = lb

			// Start everything up
			err = p.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, p.Shutdown(context.Background()))
			}()

			// Test
			dir := filepath.Join("testdata", "metrics", "consume_metrics", "single_endpoint", tc.name)

			input, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
			require.NoError(t, err)

			err = p.ConsumeMetrics(context.Background(), input)
			require.NoError(t, err)

			expectedOutput, err := golden.ReadMetrics(filepath.Join(dir, "output.yaml"))
			require.NoError(t, err)

			allOutputs := sink.AllMetrics()
			require.Len(t, allOutputs, 1)

			actualOutput := allOutputs[0]
			require.NoError(t, pmetrictest.CompareMetrics(
				expectedOutput, actualOutput,
				// We have to ignore ordering, because we do MergeMetrics() inside a map
				// iteration. And golang map iteration order is random. This means the
				// order of the merges is random
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreScopeMetricsOrder(),
				pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
			))
		})
	}
}

func TestConsumeMetrics_TripleEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	// I'm not fully satisfied with the design of this test.
	// We're hard-reliant on the implementation of the ring hash to give use the routing.
	// So if that algorithm changes, all these tests will need to be updated. In addition,
	// it's not easy to "know" what the routing *should* be. Can *can* calculate it by
	// hand, but it's very tedious.

	t.Parallel()

	testCases := []struct {
		name       string
		routingKey string
	}{
		{
			name:       "resource_service_name",
			routingKey: svcRoutingStr,
		},
		{
			name:       "resource_id",
			routingKey: resourceRoutingStr,
		},
		{
			name:       "metric_name",
			routingKey: metricNameRoutingStr,
		},
		{
			name:       "stream_id",
			routingKey: streamIDRoutingStr,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			createSettings := ts
			config := &Config{
				Resolver: ResolverSettings{
					Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}},
				},
				RoutingKey: tc.routingKey,
			}

			p, err := newMetricsExporter(createSettings, config)
			require.NoError(t, err)
			require.NotNil(t, p)

			// newMetricsExporter will internally create a loadBalancer instance which is
			// hardcoded to use OTLP exporters
			// We manually override that to use our testing sink
			sink1 := consumertest.MetricsSink{}
			sink2 := consumertest.MetricsSink{}
			sink3 := consumertest.MetricsSink{}
			componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
				if endpoint == "endpoint-1:4317" {
					return newMockMetricsExporter(sink1.ConsumeMetrics), nil
				}
				if endpoint == "endpoint-2:4317" {
					return newMockMetricsExporter(sink2.ConsumeMetrics), nil
				}
				if endpoint == "endpoint-3:4317" {
					return newMockMetricsExporter(sink3.ConsumeMetrics), nil
				}

				t.Fatalf("invalid endpoint %s", endpoint)
				return nil, errors.New("invalid endpoint")
			}

			lb, err := newLoadBalancer(ts.Logger, config, componentFactory, tb)
			require.NoError(t, err)
			require.NotNil(t, lb)

			lb.addMissingExporters(context.Background(), []string{"endpoint-1", "endpoint-2", "endpoint-3"})
			lb.res = &mockResolver{
				triggerCallbacks: true,
				onResolve: func(_ context.Context) ([]string, error) {
					return []string{"endpoint-1", "endpoint-2", "endpoint-3"}, nil
				},
			}
			p.loadBalancer = lb

			// Start everything up
			err = p.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, p.Shutdown(context.Background()))
			}()

			// Test
			dir := filepath.Join("testdata", "metrics", "consume_metrics", "triple_endpoint", tc.name)

			input, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
			require.NoError(t, err)

			err = p.ConsumeMetrics(context.Background(), input)
			require.NoError(t, err)

			expectedOutput := loadMetricsMap(t, filepath.Join(dir, "output.yaml"))

			actualOutput := map[string]pmetric.Metrics{}

			sink1Outputs := sink1.AllMetrics()
			require.LessOrEqual(t, len(sink1Outputs), 1)
			if len(sink1Outputs) == 1 {
				actualOutput["endpoint-1"] = sink1Outputs[0]
			} else {
				actualOutput["endpoint-1"] = pmetric.NewMetrics()
			}

			sink2Outputs := sink2.AllMetrics()
			require.LessOrEqual(t, len(sink2Outputs), 1)
			if len(sink2Outputs) == 1 {
				actualOutput["endpoint-2"] = sink2Outputs[0]
			} else {
				actualOutput["endpoint-2"] = pmetric.NewMetrics()
			}

			sink3Outputs := sink3.AllMetrics()
			require.LessOrEqual(t, len(sink3Outputs), 1)
			if len(sink3Outputs) == 1 {
				actualOutput["endpoint-3"] = sink3Outputs[0]
			} else {
				actualOutput["endpoint-3"] = pmetric.NewMetrics()
			}

			compareMetricsMaps(t, expectedOutput, actualOutput)
		})
	}
}

// this test validates that exporter is can concurrently change the endpoints while consuming metrics.
func TestConsumeMetrics_ConcurrentResolverChange(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	consumeStarted := make(chan struct{})
	consumeDone := make(chan struct{})

	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		// imitate a slow exporter
		te := &mockMetricsExporter{Component: mockComponent{}}
		te.ConsumeMetricsFn = func(_ context.Context, _ pmetric.Metrics) error {
			close(consumeStarted)
			time.Sleep(50 * time.Millisecond)
			return te.consumeErr
		}
		return te, nil
	}
	lb, err := newLoadBalancer(ts.Logger, simpleConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, simpleConfig())
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
		assert.NoError(t, p.ConsumeMetrics(context.Background(), simpleMetricsWithResource()))
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

func TestConsumeMetricsExporterNoEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, serviceBasedRoutingConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, endpoint2Config())
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return nil, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithServiceName())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("couldn't find the exporter for the endpoint %q", ""))
}

func TestConsumeMetricsUnexpectedExporterType(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, serviceBasedRoutingConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})
	lb.addMissingExporters(context.Background(), []string{"endpoint-2"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// test
	res := p.ConsumeMetrics(context.Background(), simpleMetricsWithServiceName())

	// verify
	assert.Error(t, res)
	assert.EqualError(t, res, fmt.Sprintf("unable to export metrics, unexpected exporter type: expected exporter.Metrics but got %T", newNopMockExporter()))
}

func TestBatchWithTwoMetrics(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	sink := new(consumertest.MetricsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockMetricsExporter(sink.ConsumeMetrics), nil
	}
	lb, err := newLoadBalancer(ts.Logger, serviceBasedRoutingConfig(), componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, serviceBasedRoutingConfig())
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb
	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	lb.addMissingExporters(context.Background(), []string{"endpoint-1"})

	td := twoServicesWithSameMetricName()

	// test
	err = p.ConsumeMetrics(context.Background(), td)

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.AllMetrics(), 2)
}

func TestRollingUpdatesWhenConsumeMetrics(t *testing.T) {
	t.Skip("Flaky Test - See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/13331")
	ts, tb := getTelemetryAssets(t)

	// this test is based on the discussion in the following issue for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/1690
	// prepare

	// simulate rolling updates, the dns resolver should resolve in the following order
	// ["127.0.0.1"] -> ["127.0.0.1", "127.0.0.2"] -> ["127.0.0.2"]
	res, err := newDNSResolver(ts.Logger, "service-1", "", 5*time.Second, 1*time.Second, tb)
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
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	lb.res = res
	p.loadBalancer = lb

	counter1 := &atomic.Int64{}
	counter2 := &atomic.Int64{}
	defaultExporters := map[string]*wrappedExporter{
		"127.0.0.1:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			counter1.Add(1)
			// simulate an unreachable backend
			time.Sleep(10 * time.Second)
			return nil
		}), "127.0.0.1"),
		"127.0.0.2:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			counter2.Add(1)
			return nil
		}), "127.0.0.2"),
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
	// keep consuming metrics every 2ms
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
					assert.NoError(t, p.ConsumeMetrics(ctx, randomMetrics(t, 1, 1, 1, 1)))
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

func randomMetrics(t require.TestingT, rmCount int, smCount int, mCount int, dpCount int) pmetric.Metrics {
	md := pmetric.NewMetrics()

	timeStamp := pcommon.Timestamp(rand.IntN(256))
	value := rand.Int64N(256)

	for i := 0; i < rmCount; i++ {
		rm := md.ResourceMetrics().AppendEmpty()
		err := rm.Resource().Attributes().FromRaw(map[string]any{
			conventions.AttributeServiceName: fmt.Sprintf("service-%d", rand.IntN(512)),
		})
		require.NoError(t, err)

		for j := 0; j < smCount; j++ {
			sm := rm.ScopeMetrics().AppendEmpty()
			scope := sm.Scope()
			scope.SetName("MyTestInstrument")
			scope.SetVersion("1.2.3")
			err = scope.Attributes().FromRaw(map[string]any{
				"scope.key": fmt.Sprintf("scope-%d", rand.IntN(512)),
			})
			require.NoError(t, err)

			for k := 0; k < mCount; k++ {
				m := sm.Metrics().AppendEmpty()
				m.SetName(fmt.Sprintf("metric.%d.test", rand.IntN(512)))

				sum := m.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)

				for l := 0; l < dpCount; l++ {
					dp := sum.DataPoints().AppendEmpty()

					dp.SetTimestamp(timeStamp)
					timeStamp += 10

					dp.SetIntValue(value)
					value += 15

					err = dp.Attributes().FromRaw(map[string]any{
						"datapoint.key": fmt.Sprintf("dp-%d", rand.IntN(512)),
					})
					require.NoError(t, err)
				}
			}
		}
	}

	return md
}

func benchConsumeMetrics(b *testing.B, routingKey string, endpointsCount int, rmCount int, smCount int, mCount int, dpCount int) {
	ts, tb := getTelemetryAssets(b)

	sink := new(consumertest.MetricsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockMetricsExporter(sink.ConsumeMetrics), nil
	}

	endpoints := []string{}
	for i := 0; i < endpointsCount; i++ {
		endpoints = append(endpoints, fmt.Sprintf("endpoint-%d", i))
	}

	config := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: endpoints},
		},
		RoutingKey: routingKey,
	}

	lb, err := newLoadBalancer(ts.Logger, config, componentFactory, tb)
	require.NotNil(b, lb)
	require.NoError(b, err)

	p, err := newMetricsExporter(ts, config)
	require.NotNil(b, p)
	require.NoError(b, err)

	p.loadBalancer = lb

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(b, err)

	md := randomMetrics(b, rmCount, smCount, mCount, dpCount)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err = p.ConsumeMetrics(context.Background(), md)
		require.NoError(b, err)
	}

	b.StopTimer()
	err = p.Shutdown(context.Background())
	require.NoError(b, err)
}

func BenchmarkConsumeMetrics(b *testing.B) {
	testCases := []struct {
		routingKey string
	}{
		{
			routingKey: svcRoutingStr,
		},
		{
			routingKey: resourceRoutingStr,
		},
		{
			routingKey: metricNameRoutingStr,
		},
		{
			routingKey: streamIDRoutingStr,
		},
	}

	for _, tc := range testCases {
		b.Run(tc.routingKey, func(b *testing.B) {
			for _, endpointCount := range []int{1, 5, 10} {
				for _, rmCount := range []int{1, 3} {
					for _, smCount := range []int{1, 3} {
						for _, totalMCount := range []int{100, 500, 1000} {
							mCount := totalMCount / smCount / rmCount
							dpCount := 2

							b.Run(fmt.Sprintf("%dE_%dRM_%dSM_%dM", endpointCount, rmCount, smCount, mCount), func(b *testing.B) {
								benchConsumeMetrics(b, tc.routingKey, endpointCount, rmCount, smCount, mCount, dpCount)
							})
						}
					}
				}
			}
		})
	}
}

func endpoint2Config() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
		RoutingKey: "service",
	}
}

func resourceBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
		RoutingKey: resourceRoutingStr,
	}
}

func metricNameBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
		RoutingKey: metricNameRoutingStr,
	}
}

func simpleMetricsWithServiceName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName1)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func simpleMetricsWithResource() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName1)
	rmetrics.Resource().Attributes().PutStr(keyAttr1, valueAttr1)
	rmetrics.Resource().Attributes().PutInt(keyAttr2, valueAttr2)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func twoServicesWithSameMetricName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)
	rs1 := metrics.ResourceMetrics().AppendEmpty()
	rs1.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName1)
	appendSimpleMetricWithID(rs1, signal1Name)
	rs2 := metrics.ResourceMetrics().AppendEmpty()
	rs2.Resource().Attributes().PutStr(conventions.AttributeServiceName, serviceName2)
	appendSimpleMetricWithID(rs2, signal1Name)
	return metrics
}

func appendSimpleMetricWithID(dest pmetric.ResourceMetrics, id string) {
	dest.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(id)
}

type mockMetricsExporter struct {
	component.Component
	ConsumeMetricsFn func(ctx context.Context, td pmetric.Metrics) error
	consumeErr       error
}

func newMockMetricsExporter(consumeMetricsFn func(ctx context.Context, td pmetric.Metrics) error) exporter.Metrics {
	return &mockMetricsExporter{
		Component:        mockComponent{},
		ConsumeMetricsFn: consumeMetricsFn,
	}
}

func newNopMockMetricsExporter() exporter.Metrics {
	return &mockMetricsExporter{Component: mockComponent{}}
}

func (e *mockMetricsExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *mockMetricsExporter) Shutdown(context.Context) error {
	e.consumeErr = errors.New("exporter is shut down")
	return nil
}

func (e *mockMetricsExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if e.ConsumeMetricsFn == nil {
		return e.consumeErr
	}
	return e.ConsumeMetricsFn(ctx, md)
}
