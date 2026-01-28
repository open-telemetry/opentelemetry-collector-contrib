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

	json "github.com/goccy/go-json"
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
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	serviceName1      = "service-name-01"
	serviceName2      = "service-name-02"
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
			res := p.Start(t.Context(), componenttest.NewNopHost())
			defer func() {
				require.NoError(t, p.Shutdown(t.Context()))
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
	res := p.Shutdown(t.Context())

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
		b, err = json.Marshal(data)
		require.NoError(t, err)

		unmarshaller := &pmetric.JSONUnmarshaler{}
		md, err := unmarshaller.UnmarshalMetrics(b)
		require.NoError(t, err)

		expectedOutput[key] = md
	}

	return expectedOutput
}

func compareMetricsMaps(t *testing.T, expected, actual map[string]pmetric.Metrics) {
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

			output, errs := splitMetricsByResourceServiceName(input)
			require.Nil(t, errs)
			compareMetricsMaps(t, expectedOutput, output)
		})
	}
}

func TestSplitMetricsByResourceServiceNameFailsIfMissingServiceNameAttribute(t *testing.T) {
	t.Parallel()

	input, err := golden.ReadMetrics(filepath.Join("testdata", "metrics", "split_metrics", "missing_service_name", "input.yaml"))
	require.NoError(t, err)

	_, errs := splitMetricsByResourceServiceName(input)
	require.NotNil(t, errs)
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
					Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
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

			lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
			lb.res = &mockResolver{
				triggerCallbacks: true,
				onResolve: func(_ context.Context) ([]string, error) {
					return []string{"endpoint-1"}, nil
				},
			}
			p.loadBalancer = lb

			// Start everything up
			err = p.Start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, p.Shutdown(t.Context()))
			}()

			// Test
			dir := filepath.Join("testdata", "metrics", "consume_metrics", "single_endpoint", tc.name)

			input, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
			require.NoError(t, err)

			err = p.ConsumeMetrics(t.Context(), input)
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

func TestConsumeMetrics_SingleEndpointNoServiceName(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	createSettings := ts
	config := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
		},
		RoutingKey: svcRoutingStr,
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

	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}
	p.loadBalancer = lb

	// Start everything up
	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Test
	dir := filepath.Join("testdata", "metrics", "consume_metrics", "single_endpoint", "resource_no_service_name")

	input, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
	require.NoError(t, err)

	err = p.ConsumeMetrics(t.Context(), input)
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
					Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2", "endpoint-3"}}),
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

			lb.addMissingExporters(t.Context(), []string{"endpoint-1", "endpoint-2", "endpoint-3"})
			lb.res = &mockResolver{
				triggerCallbacks: true,
				onResolve: func(_ context.Context) ([]string, error) {
					return []string{"endpoint-1", "endpoint-2", "endpoint-3"}, nil
				},
			}
			p.loadBalancer = lb

			// Start everything up
			err = p.Start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, p.Shutdown(t.Context()))
			}()

			// Test
			dir := filepath.Join("testdata", "metrics", "consume_metrics", "triple_endpoint", tc.name)

			input, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
			require.NoError(t, err)

			err = p.ConsumeMetrics(t.Context(), input)
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	go func() {
		assert.NoError(t, p.ConsumeMetrics(t.Context(), simpleMetricsWithResource()))
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

func TestConsumeMetrics_DNSResolverRetriesOnUnreachableEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: "", Quarantine: QuarantineSettings{Enabled: true}}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newMetricsExporter(ts, simpleConfig())
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
		"endpoint-1:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			return nil
		}), "endpoint-2:4317"),
	}

	// test - first call should fail by retrying with endpoint-2
	res := p.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())
	require.NoError(t, res)

	// We want to verify that metrics are consumed by "endpoint-2" exporter, which uses consumeWithRetry.
	// To do so, we can provide a spy function and check whether it is called.
	var consumedBySecondEndpoint atomic.Bool
	lb.exporters["endpoint-2:4317"] = newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
		consumedBySecondEndpoint.Store(true)
		return nil
	}), "endpoint-2:4317")

	// Re-run test with the spy.
	require.NoError(t, p.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName()))
	require.True(t, consumedBySecondEndpoint.Load(), "metrics should be consumed by the second endpoint")
}

func TestConsumeMetrics_DNSResolverRetriesExhausted(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := &Config{
		Resolver: ResolverSettings{
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: "", Quarantine: QuarantineSettings{Enabled: true}}),
		},
	}
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newMetricsExporter(ts, simpleConfig())
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
		"endpoint-1:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			return errors.New("endpoint unreachable")
		}), "endpoint-2:4317"),
	}

	// test - first call should fail by retrying with endpoint-2
	res := p.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())
	require.Error(t, res)
	require.EqualError(t, res, "all endpoints were tried and failed: map[endpoint-1:true endpoint-2:true]")
}

func TestConsumeMetrics_DNSResolverQuarantineWithParentExporterBackoffEnabled(t *testing.T) {
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
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newMetricsExporter(ts, cfg)
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
		"endpoint-1:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			endpoint1Attempts.Add(1)
			totalAttempts.Add(1)
			timesMutex.Lock()
			attemptTimes = append(attemptTimes, time.Now())
			timesMutex.Unlock()
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
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
	retryExporter, err := exporterhelper.NewMetrics(
		t.Context(),
		ts,
		cfg,
		p.ConsumeMetrics,
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
	res := retryExporter.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())
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

func TestConsumeMetrics_DNSResolverQuarantineWithParentExporterBackoffDisabled(t *testing.T) {
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
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newMetricsExporter(ts, cfg)
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
		"endpoint-1:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			endpoint1Attempts.Add(1)
			totalAttempts.Add(1)
			timesMutex.Lock()
			attemptTimes = append(attemptTimes, time.Now())
			timesMutex.Unlock()
			return errors.New("endpoint unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			endpoint2Attempts.Add(1)
			totalAttempts.Add(1)
			timesMutex.Lock()
			attemptTimes = append(attemptTimes, time.Now())
			timesMutex.Unlock()
			return errors.New("endpoint unreachable")
		}), "endpoint-2:4317"),
	}

	// Wrap with exporterhelper without backoff
	retryExporter, err := exporterhelper.NewMetrics(
		t.Context(),
		ts,
		cfg,
		p.ConsumeMetrics,
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
	res := retryExporter.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())
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
func TestConsumeMetrics_DNSResolverQuarantineWithParentExporterBackoffEnabled_PartialRecovery(t *testing.T) {
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
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newMetricsExporter(ts, cfg)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.loadBalancer = lb

	// Endpoint-1 fails initially but recovers after 150ms
	// Endpoint-2 always fails
	endpoint1Attempts := &atomic.Int64{}
	endpoint2Attempts := &atomic.Int64{}
	var startTime time.Time

	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			endpoint1Attempts.Add(1)
			// Simulate recovery after 150ms
			if time.Since(startTime) > 150*time.Millisecond {
				t.Log("endpoint-1 recovered")
				return nil
			}
			t.Log("endpoint-1 temporarily unreachable")
			return errors.New("endpoint temporarily unreachable")
		}), "endpoint-1:4317"),
		"endpoint-2:4317": newWrappedExporter(newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
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
	retryExporter, err := exporterhelper.NewMetrics(
		t.Context(),
		ts,
		cfg,
		p.ConsumeMetrics,
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
	res := retryExporter.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())

	// Should succeed after endpoint-1 recovers
	require.NoError(t, res, "should succeed after endpoint-1 recovers")

	t.Logf("Endpoint-1 attempts: %d, Endpoint-2 attempts: %d",
		endpoint1Attempts.Load(), endpoint2Attempts.Load())

	// Both endpoints should have been tried (quarantine logic)
	require.Greater(t, endpoint1Attempts.Load(), int64(1), "endpoint-1 should be retried after quarantine period")
	require.Positive(t, endpoint2Attempts.Load(), "endpoint-2 should be tried at least once")
}

func TestConsumeMetrics_DNSResolverQuarantineWithSubExporterBackoffEnabled(t *testing.T) {
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
		return newNopMockMetricsExporter(), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NotNil(t, lb)
	require.NoError(t, err)

	// Set up the hash ring with the test endpoints
	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	p, err := newMetricsExporter(ts, cfg)
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
	mockExporter1 := newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
		endpoint1Attempts.Add(1)
		totalAttempts.Add(1)
		timesMutex.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		timesMutex.Unlock()
		return errors.New("endpoint unreachable")
	})
	mockExporter2 := newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
		endpoint2Attempts.Add(1)
		totalAttempts.Add(1)
		timesMutex.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		timesMutex.Unlock()
		return errors.New("endpoint unreachable")
	})

	// Wrap with exporterhelper to apply retry configuration from cfg.Protocol.OTLP.RetryConfig
	wrappedExp1, err := exporterhelper.NewMetrics(
		t.Context(),
		exporter.Settings{
			ID:                component.NewIDWithName(component.MustNewType("otlp"), "endpoint-1"),
			TelemetrySettings: ts.TelemetrySettings,
		},
		&cfg.Protocol.OTLP,
		mockExporter1.ConsumeMetrics,
		exporterhelper.WithRetry(cfg.Protocol.OTLP.RetryConfig),
		exporterhelper.WithStart(mockExporter1.Start),
		exporterhelper.WithShutdown(mockExporter1.Shutdown),
	)
	require.NoError(t, err)

	wrappedExp2, err := exporterhelper.NewMetrics(
		t.Context(),
		exporter.Settings{
			ID:                component.NewIDWithName(component.MustNewType("otlp"), "endpoint-2"),
			TelemetrySettings: ts.TelemetrySettings,
		},
		&cfg.Protocol.OTLP,
		mockExporter2.ConsumeMetrics,
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
	res := p.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())
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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	res := p.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())

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
	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-2"})
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return []string{"endpoint-1", "endpoint-2"}, nil
		},
	}
	p.loadBalancer = lb

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// test
	res := p.ConsumeMetrics(t.Context(), simpleMetricsWithServiceName())

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
	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), []string{"endpoint-1"})

	td := twoServicesWithSameMetricName()

	// test
	err = p.ConsumeMetrics(t.Context(), td)

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
	res, err := newDNSResolver(ts.Logger, "service-1", "", 5*time.Second, 1*time.Second, nil, tb)
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
			DNS: configoptional.Some(DNSResolver{Hostname: "service-1", Port: ""}),
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

func randomMetrics(t require.TestingT, rmCount, smCount, mCount, dpCount int) pmetric.Metrics {
	md := pmetric.NewMetrics()

	timeStamp := pcommon.Timestamp(rand.IntN(256))
	value := rand.Int64N(256)

	for range rmCount {
		rm := md.ResourceMetrics().AppendEmpty()
		err := rm.Resource().Attributes().FromRaw(map[string]any{
			"service.name": fmt.Sprintf("service-%d", rand.IntN(512)),
		})
		require.NoError(t, err)

		for range smCount {
			sm := rm.ScopeMetrics().AppendEmpty()
			scope := sm.Scope()
			scope.SetName("MyTestInstrument")
			scope.SetVersion("1.2.3")
			err = scope.Attributes().FromRaw(map[string]any{
				"scope.key": fmt.Sprintf("scope-%d", rand.IntN(512)),
			})
			require.NoError(t, err)

			for range mCount {
				m := sm.Metrics().AppendEmpty()
				m.SetName(fmt.Sprintf("metric.%d.test", rand.IntN(512)))

				sum := m.SetEmptySum()
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				sum.SetIsMonotonic(true)

				for range dpCount {
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

func benchConsumeMetrics(b *testing.B, routingKey string, endpointsCount, rmCount, smCount, mCount, dpCount int) {
	ts, tb := getTelemetryAssets(b)

	sink := new(consumertest.MetricsSink)
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockMetricsExporter(sink.ConsumeMetrics), nil
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

	p, err := newMetricsExporter(ts, config)
	require.NotNil(b, p)
	require.NoError(b, err)

	p.loadBalancer = lb

	err = p.Start(b.Context(), componenttest.NewNopHost())
	require.NoError(b, err)

	md := randomMetrics(b, rmCount, smCount, mCount, dpCount)

	for b.Loop() {
		err = p.ConsumeMetrics(b.Context(), md)
		require.NoError(b, err)
	}

	b.StopTimer()
	err = p.Shutdown(b.Context())
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
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}}),
		},
		RoutingKey: "service",
	}
}

func resourceBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}}),
		},
		RoutingKey: resourceRoutingStr,
	}
}

func metricNameBasedRoutingConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}}),
		},
		RoutingKey: metricNameRoutingStr,
	}
}

func simpleMetricsWithServiceName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr("service.name", serviceName1)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func simpleMetricsWithResource() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr("service.name", serviceName1)
	rmetrics.Resource().Attributes().PutStr(keyAttr1, valueAttr1)
	rmetrics.Resource().Attributes().PutInt(keyAttr2, valueAttr2)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func twoServicesWithSameMetricName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)
	rs1 := metrics.ResourceMetrics().AppendEmpty()
	rs1.Resource().Attributes().PutStr("service.name", serviceName1)
	appendSimpleMetricWithID(rs1, signal1Name)
	rs2 := metrics.ResourceMetrics().AppendEmpty()
	rs2.Resource().Attributes().PutStr("service.name", serviceName2)
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

func (*mockMetricsExporter) Capabilities() consumer.Capabilities {
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
