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
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
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
				p.loadBalancer.res = &mockResolver{}
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

func expectedMetricsByEndpoint(t *testing.T, md pmetric.Metrics, routingKey string, routingAttrs []string, ring *hashRing, endpoints ...string) map[string]pmetric.Metrics {
	expected := make(map[string]pmetric.Metrics, len(endpoints))
	for _, endpoint := range endpoints {
		expected[endpoint] = pmetric.NewMetrics()
	}

	var batches map[string]pmetric.Metrics
	switch routingKey {
	case svcRoutingStr:
		var errs []error
		batches, errs = splitMetricsByResourceServiceName(md)
		require.Empty(t, errs)
	case resourceRoutingStr:
		batches = splitMetricsByResourceID(md)
	case metricNameRoutingStr:
		batches = splitMetricsByMetricName(md)
	case streamIDRoutingStr:
		batches = splitMetricsByStreamID(md)
	case attrRoutingStr:
		batches = splitMetricsByAttributes(md, routingAttrs)
	default:
		t.Fatalf("unexpected routing key %q", routingKey)
	}

	for routingID, batch := range batches {
		endpoint := ring.endpointFor([]byte(routingID))
		metrics.Merge(expected[endpoint], batch)
	}
	return expected
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
		{
			name: "basic_attributes",
			splitFunc: func(md pmetric.Metrics) map[string]pmetric.Metrics {
				return splitMetricsByAttributes(md, []string{"resource_key", "scope_key", "aaa"})
			},
		},
		{
			name: "attributes_resource_only",
			splitFunc: func(md pmetric.Metrics) map[string]pmetric.Metrics {
				return splitMetricsByAttributes(md, []string{"resource_key"})
			},
		},
		{
			name: "attributes_scope_only",
			splitFunc: func(md pmetric.Metrics) map[string]pmetric.Metrics {
				return splitMetricsByAttributes(md, []string{"scope_key"})
			},
		},
		{
			name: "attributes_datapoint_only",
			splitFunc: func(md pmetric.Metrics) map[string]pmetric.Metrics {
				return splitMetricsByAttributes(md, []string{"aaa"})
			},
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

func TestSplitMetricsByAttributes_StableEncodingAvoidsConcatenationCollisions(t *testing.T) {
	md := pmetric.NewMetrics()

	buildResourceMetric := func(aValue, bValue string) {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("a", aValue)
		rm.Resource().Attributes().PutStr("b", bValue)

		sm := rm.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("test.metric")

		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(false)

		dp := sum.DataPoints().AppendEmpty()
		dp.SetIntValue(1)
	}

	buildResourceMetric("foo", "bar")
	buildResourceMetric("foob", "ar")

	out := splitMetricsByAttributes(md, []string{"a", "b"})
	require.Len(t, out, 2)
	assert.Contains(t, out, "a=foo|b=bar|")
	assert.Contains(t, out, "a=foob|b=ar|")
}

func TestSplitMetricsByAttributes_StableEncodingIncludesMissingAttributes(t *testing.T) {
	md := pmetric.NewMetrics()

	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("resource_key", "res1")

	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test.metric")

	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sum.SetIsMonotonic(false)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetIntValue(1)
	dp.Attributes().PutStr("aaa", "dp1")

	out := splitMetricsByAttributes(md, []string{"resource_key", "missing", "aaa"})
	require.Len(t, out, 1)
	assert.Contains(t, out, "resource_key=res1|missing=|aaa=dp1|")
}

func TestSplitMetricsByAttributes_NonStringValues(t *testing.T) {
	md := pmetric.NewMetrics()

	buildResourceMetric := func(shard int64) {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutInt("shard", shard)

		sm := rm.ScopeMetrics().AppendEmpty()
		metric := sm.Metrics().AppendEmpty()
		metric.SetName("test.metric")

		sum := metric.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(false)

		dp := sum.DataPoints().AppendEmpty()
		dp.SetIntValue(1)
	}

	buildResourceMetric(1)
	buildResourceMetric(2)

	out := splitMetricsByAttributes(md, []string{"shard"})
	require.Len(t, out, 2)
	assert.Contains(t, out, "shard=1|")
	assert.Contains(t, out, "shard=2|")
}

func TestConsumeMetrics_SingleEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	t.Parallel()

	testCases := []struct {
		name              string
		routingKey        string
		routingAttributes []string
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
		{
			name:              "attributes",
			routingKey:        attrRoutingStr,
			routingAttributes: []string{"resource_key", "scope_key", "aaa"},
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
				RoutingKey:        tc.routingKey,
				RoutingAttributes: tc.routingAttributes,
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
		name              string
		routingKey        string
		routingAttributes []string
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
		{
			name:              "attributes",
			routingKey:        attrRoutingStr,
			routingAttributes: []string{"resource_key", "scope_key", "aaa"},
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
				RoutingKey:        tc.routingKey,
				RoutingAttributes: tc.routingAttributes,
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

			expectedOutput := expectedMetricsByEndpoint(
				t,
				input,
				tc.routingKey,
				tc.routingAttributes,
				lb.ring,
				"endpoint-1",
				"endpoint-2",
				"endpoint-3",
			)

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
	merged := pmetric.NewMetrics()
	for _, output := range sink.AllMetrics() {
		metrics.Merge(merged, output)
	}
	require.NoError(t, pmetrictest.CompareMetrics(
		td,
		merged,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

// metricsRoutedToBothEndpoints finds one service name that the given ring routes to each
// of endpointA and endpointB, so a test can control which backend a resource lands on.
func metricsRoutedToBothEndpoints(t *testing.T, ring *hashRing, endpointA, endpointB string) (svcA, svcB string) {
	for i := range 1000 {
		name := fmt.Sprintf("svc-%d", i)
		switch ring.endpointFor([]byte(name)) {
		case endpointA:
			if svcA == "" {
				svcA = name
			}
		case endpointB:
			if svcB == "" {
				svcB = name
			}
		}
		if svcA != "" && svcB != "" {
			break
		}
	}
	require.NotEmpty(t, svcA, "no service name routes to %q", endpointA)
	require.NotEmpty(t, svcB, "no service name routes to %q", endpointB)
	return svcA, svcB
}

// mutateMetricNames overwrites the name of every metric in md. Used after extracting data
// from a consumererror to prove it is a deep copy: mutating it must never be observable
// through the caller's original input.
func mutateMetricNames(md pmetric.Metrics, value string) {
	rms := md.ResourceMetrics()
	for i := range rms.Len() {
		sms := rms.At(i).ScopeMetrics()
		for j := range sms.Len() {
			ms := sms.At(j).Metrics()
			for k := range ms.Len() {
				ms.At(k).SetName(value)
			}
		}
	}
}

// metricNamesRoutedToBothEndpoints finds one metric name that the given ring routes to each
// of endpointA and endpointB, so a test can control which backend a metric lands on
// independently of its resource attributes.
func metricNamesRoutedToBothEndpoints(t *testing.T, ring *hashRing, endpointA, endpointB string) (nameA, nameB string) {
	for i := range 1000 {
		name := fmt.Sprintf("metric-%d", i)
		switch ring.endpointFor([]byte(name)) {
		case endpointA:
			if nameA == "" {
				nameA = name
			}
		case endpointB:
			if nameB == "" {
				nameB = name
			}
		}
		if nameA != "" && nameB != "" {
			break
		}
	}
	require.NotEmpty(t, nameA, "no metric name routes to %q", endpointA)
	require.NotEmpty(t, nameB, "no metric name routes to %q", endpointB)
	return nameA, nameB
}

// TestConsumeMetrics_PartialFailureReturnsFailedSubset verifies that when one of two
// backends fails, ConsumeMetrics returns a consumererror.Metrics embedding only the
// resource metrics destined for the failed backend, and leaves the caller's input metrics
// untouched (#50437).
func TestConsumeMetrics_PartialFailureReturnsFailedSubset(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	badErr := errors.New("endpoint-bad: unreachable")
	var goodCalls, badCalls int
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		bad := endpoint == endpointWithPort("endpoint-bad")
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			if bad {
				badCalls++
				return badErr
			}
			goodCalls++
			return nil
		}), nil
	}

	endpoints := []string{"endpoint-good", "endpoint-bad"}
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: endpoints}),
		},
		RoutingKey: "service",
	}

	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	require.Equal(t, svcRouting, p.routingKey)

	lb.addMissingExporters(t.Context(), endpoints)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	goodSvc, badSvc := metricsRoutedToBothEndpoints(t, lb.ring, "endpoint-good", "endpoint-bad")

	md := pmetric.NewMetrics()
	goodRM := md.ResourceMetrics().AppendEmpty()
	goodRM.Resource().Attributes().PutStr("service.name", goodSvc)
	appendSimpleMetricWithID(goodRM, "good-metric")
	badRM := md.ResourceMetrics().AppendEmpty()
	badRM.Resource().Attributes().PutStr("service.name", badSvc)
	appendSimpleMetricWithID(badRM, "bad-metric")

	originalCopy := pmetric.NewMetrics()
	md.CopyTo(originalCopy)

	res := p.ConsumeMetrics(t.Context(), md)

	require.Error(t, res)
	var partial consumererror.Metrics
	require.True(t, errors.As(res, &partial), "error must be a consumererror.Metrics")
	failed := partial.Data()

	expectedFailed := pmetric.NewMetrics()
	failedRM := expectedFailed.ResourceMetrics().AppendEmpty()
	failedRM.Resource().Attributes().PutStr("service.name", badSvc)
	appendSimpleMetricWithID(failedRM, "bad-metric")
	require.NoError(t, pmetrictest.CompareMetrics(expectedFailed, failed))

	assert.Equal(t, 1, goodCalls)
	assert.Equal(t, 1, badCalls)

	// Mutating the extracted failed data must not be observable on the original input:
	// proves the embedded data is a deep copy, not an alias into md's buffers.
	mutateMetricNames(failed, "mutated")

	// The original input must be left untouched.
	require.NoError(t, pmetrictest.CompareMetrics(originalCopy, md))
}

// TestConsumeMetrics_TotalFailureReturnsFullData verifies that when every backend fails,
// the embedded failed data covers the full input, and the caller's input metrics are left
// untouched (#50437).
func TestConsumeMetrics_TotalFailureReturnsFullData(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	err1 := errors.New("endpoint-1: unreachable")
	err2 := errors.New("endpoint-2: unreachable")
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		failWith := err1
		if endpoint == endpointWithPort("endpoint-2") {
			failWith = err2
		}
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			return failWith
		}), nil
	}

	cfg := serviceBasedRoutingConfig()
	endpoints := []string{"endpoint-1", "endpoint-2"}

	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), endpoints)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	svc1, svc2 := metricsRoutedToBothEndpoints(t, lb.ring, "endpoint-1", "endpoint-2")

	md := pmetric.NewMetrics()
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", svc1)
	appendSimpleMetricWithID(rm1, "m1")
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", svc2)
	appendSimpleMetricWithID(rm2, "m2")

	originalCopy := pmetric.NewMetrics()
	md.CopyTo(originalCopy)

	res := p.ConsumeMetrics(t.Context(), md)

	require.Error(t, res)
	var partial consumererror.Metrics
	require.True(t, errors.As(res, &partial), "error must be a consumererror.Metrics")
	failed := partial.Data()

	require.NoError(t, pmetrictest.CompareMetrics(originalCopy, failed,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))

	// Mutating the extracted failed data must not be observable on the original input:
	// proves the embedded data is a deep copy, not an alias into md's buffers.
	mutateMetricNames(failed, "mutated")

	// The original input must be left untouched.
	require.NoError(t, pmetrictest.CompareMetrics(originalCopy, md,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

// TestConsumeMetrics_MixedPermanentAndRetryableFailure verifies that when one backend fails
// permanently and another fails retryably, ConsumeMetrics returns an error that is NOT
// permanent - so a parent retry sender still fires - and that the embedded data covers BOTH
// backends' resource metrics. The permanent backend's metrics stay embedded (rather than
// being dropped) because dropping them would let a later successful retry of the retryable
// remainder report the whole original request as sent, hiding the permanent loss (#50437).
func TestConsumeMetrics_MixedPermanentAndRetryableFailure(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	retryableErr := errors.New("endpoint-retryable: unreachable")
	permanentErr := consumererror.NewPermanent(errors.New("endpoint-permanent: bad data"))
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		permanent := endpoint == endpointWithPort("endpoint-permanent")
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			if permanent {
				return permanentErr
			}
			return retryableErr
		}), nil
	}

	endpoints := []string{"endpoint-retryable", "endpoint-permanent"}
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: endpoints}),
		},
		RoutingKey: "service",
	}

	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	require.Equal(t, svcRouting, p.routingKey)

	lb.addMissingExporters(t.Context(), endpoints)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	retryableSvc, permanentSvc := metricsRoutedToBothEndpoints(t, lb.ring, "endpoint-retryable", "endpoint-permanent")

	md := pmetric.NewMetrics()
	retryableRM := md.ResourceMetrics().AppendEmpty()
	retryableRM.Resource().Attributes().PutStr("service.name", retryableSvc)
	appendSimpleMetricWithID(retryableRM, "retryable-metric")
	permanentRM := md.ResourceMetrics().AppendEmpty()
	permanentRM.Resource().Attributes().PutStr("service.name", permanentSvc)
	appendSimpleMetricWithID(permanentRM, "permanent-metric")

	res := p.ConsumeMetrics(t.Context(), md)

	require.Error(t, res)
	assert.False(t, consumererror.IsPermanent(res), "a mixed failure must stay retryable so the retry sender still fires")

	var partial consumererror.Metrics
	require.True(t, errors.As(res, &partial), "error must be a consumererror.Metrics")
	failed := partial.Data()

	expectedFailed := pmetric.NewMetrics()
	retryableFailedRM := expectedFailed.ResourceMetrics().AppendEmpty()
	retryableFailedRM.Resource().Attributes().PutStr("service.name", retryableSvc)
	appendSimpleMetricWithID(retryableFailedRM, "retryable-metric")
	permanentFailedRM := expectedFailed.ResourceMetrics().AppendEmpty()
	permanentFailedRM.Resource().Attributes().PutStr("service.name", permanentSvc)
	appendSimpleMetricWithID(permanentFailedRM, "permanent-metric")
	require.NoError(t, pmetrictest.CompareMetrics(expectedFailed, failed, pmetrictest.IgnoreResourceMetricsOrder()))
}

// TestConsumeMetrics_AllPermanentFailureIsPermanent verifies that when every backend fails
// permanently, the returned error still satisfies consumererror.IsPermanent - so the parent
// retry sender drops the batch instead of retrying forever - and the embedded failed data
// covers every permanently-failed backend's resource metrics (#50437).
func TestConsumeMetrics_AllPermanentFailureIsPermanent(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	err1 := consumererror.NewPermanent(errors.New("endpoint-1: bad data"))
	err2 := consumererror.NewPermanent(errors.New("endpoint-2: bad data"))
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		failWith := err1
		if endpoint == endpointWithPort("endpoint-2") {
			failWith = err2
		}
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			return failWith
		}), nil
	}

	cfg := serviceBasedRoutingConfig()
	endpoints := []string{"endpoint-1", "endpoint-2"}

	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)

	lb.addMissingExporters(t.Context(), endpoints)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	svc1, svc2 := metricsRoutedToBothEndpoints(t, lb.ring, "endpoint-1", "endpoint-2")

	md := pmetric.NewMetrics()
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("service.name", svc1)
	appendSimpleMetricWithID(rm1, "m1")
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("service.name", svc2)
	appendSimpleMetricWithID(rm2, "m2")

	originalCopy := pmetric.NewMetrics()
	md.CopyTo(originalCopy)

	res := p.ConsumeMetrics(t.Context(), md)

	require.Error(t, res)
	assert.True(t, consumererror.IsPermanent(res), "an all-permanent failure must stay permanent so the retry sender drops it")

	var partial consumererror.Metrics
	require.True(t, errors.As(res, &partial), "error must be a consumererror.Metrics")
	failed := partial.Data()

	require.NoError(t, pmetrictest.CompareMetrics(originalCopy, failed,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

// TestConsumeMetrics_FailedDataPreservesDistinctSchemaURLs verifies that when two
// ResourceMetrics share an identical Resource (and their ScopeMetrics share an identical
// Scope) but carry different resource- and scope-level SchemaUrl values, and both are routed
// to failing endpoints, the embedded failed data keeps them as two separate containers with
// their own SchemaUrl, rather than collapsing through metrics.Merge, whose identity model
// does not compare SchemaUrl (#50437).
func TestConsumeMetrics_FailedDataPreservesDistinctSchemaURLs(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	err1 := errors.New("endpoint-1: unreachable")
	err2 := errors.New("endpoint-2: unreachable")
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		failWith := err1
		if endpoint == endpointWithPort("endpoint-2") {
			failWith = err2
		}
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			return failWith
		}), nil
	}

	endpoints := []string{"endpoint-1", "endpoint-2"}
	cfg := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: endpoints}),
		},
		RoutingKey: metricNameRoutingStr,
	}

	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	require.Equal(t, metricNameRouting, p.routingKey)

	lb.addMissingExporters(t.Context(), endpoints)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(context.Context) ([]string, error) {
			return endpoints, nil
		},
	}
	p.loadBalancer = lb

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	name1, name2 := metricNamesRoutedToBothEndpoints(t, lb.ring, "endpoint-1", "endpoint-2")

	md := pmetric.NewMetrics()
	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("resattr", "shared")
	rm1.SetSchemaUrl("https://schema.example.com/resource/v1")
	sm1 := rm1.ScopeMetrics().AppendEmpty()
	sm1.Scope().SetName("shared-scope")
	sm1.SetSchemaUrl("https://schema.example.com/scope/v1")
	sm1.Metrics().AppendEmpty().SetName(name1)
	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("resattr", "shared")
	rm2.SetSchemaUrl("https://schema.example.com/resource/v2")
	sm2 := rm2.ScopeMetrics().AppendEmpty()
	sm2.Scope().SetName("shared-scope")
	sm2.SetSchemaUrl("https://schema.example.com/scope/v2")
	sm2.Metrics().AppendEmpty().SetName(name2)

	res := p.ConsumeMetrics(t.Context(), md)

	require.Error(t, res)
	var partial consumererror.Metrics
	require.True(t, errors.As(res, &partial), "error must be a consumererror.Metrics")
	failed := partial.Data()

	require.Equal(t, 2, failed.ResourceMetrics().Len(), "distinct-schema resource metrics must not collapse into one")

	type schemaPair struct{ resourceSchema, scopeSchema string }
	gotByMetric := map[string]schemaPair{}
	for i := range failed.ResourceMetrics().Len() {
		rm := failed.ResourceMetrics().At(i)
		for j := range rm.ScopeMetrics().Len() {
			sm := rm.ScopeMetrics().At(j)
			for k := range sm.Metrics().Len() {
				gotByMetric[sm.Metrics().At(k).Name()] = schemaPair{rm.SchemaUrl(), sm.SchemaUrl()}
			}
		}
	}
	assert.Equal(t, map[string]schemaPair{
		name1: {"https://schema.example.com/resource/v1", "https://schema.example.com/scope/v1"},
		name2: {"https://schema.example.com/resource/v2", "https://schema.example.com/scope/v2"},
	}, gotByMetric)
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
