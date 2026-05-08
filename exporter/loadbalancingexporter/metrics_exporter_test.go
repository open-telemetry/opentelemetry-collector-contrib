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
	"slices"
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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadatatest"
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
	serviceNameKey    = "service.name"
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

func TestConsumeMetricsCentralQueueEnqueuesCompressedByRoutingKey(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	p := &metricExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		routingKey:   svcRouting,
	}
	p.started.Store(true)

	md := simpleMetricsWithServiceName()
	require.NoError(t, p.ConsumeMetrics(t.Context(), md))
	require.Equal(t, 1, p.centralQueue.len())
	require.Positive(t, p.centralQueue.compressedBytes())

	lease, err := p.centralQueue.lease(t.Context())
	require.NoError(t, err)
	defer lease.done()
	require.Equal(t, signalKindMetrics, lease.item.signal)
	require.Equal(t, []byte(serviceName1), lease.item.routingKey)
	require.Equal(t, len(lease.item.payload), cap(lease.item.payload))

	decoded, err := decodeCentralQueueMetricsItem(lease.item, codec)
	require.NoError(t, err)
	require.Equal(t, md.DataPointCount(), decoded.DataPointCount())
}

func TestMetricsCentralQueueDropsPermanentExportError(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	endpoint := "endpoint-1:4317"
	var calls atomic.Int64
	p := &metricExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{endpoint}),
			exporters: map[string]*wrappedExporter{endpoint: newWrappedExporter(newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
				calls.Add(1)
				return consumererror.NewPermanent(errors.New("bad metrics"))
			}), endpoint)},
			endpointHealth: newEndpointHealthManager(endpointHealthSettings{}),
		},
		telemetry: tb,
		logger:    ts.Logger,
	}

	ctx, cancel := context.WithCancel(t.Context())
	p.centralWG.Add(1)
	go p.runCentralQueue(ctx)
	item, err := newCentralQueueMetricsItem([]byte("route-a"), singleDataPointMetric("permanent"), codec, time.Now())
	require.NoError(t, err)
	require.NoError(t, p.centralQueue.enqueue(item))

	require.Eventually(t, func() bool {
		return calls.Load() == 1 && p.centralQueue.len() == 0
	}, 2*time.Second, 20*time.Millisecond)
	cancel()
	waitErr := waitForInflight(t.Context(), &p.centralWG)
	require.NoError(t, waitErr)
}

func TestMetricsCentralQueueShutdownTimeoutSkipsLoadBalancerShutdown(t *testing.T) {
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })
	var resolverShutdown atomic.Bool
	p := &metricExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			res: &mockResolver{onShutdown: func(context.Context) error {
				resolverShutdown.Store(true)
				return nil
			}},
		},
	}
	p.started.Store(true)
	p.centralWG.Add(1)
	t.Cleanup(p.centralWG.Done)

	shutdownCtx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	require.Error(t, p.Shutdown(shutdownCtx))
	require.False(t, resolverShutdown.Load())
}

func TestMetricsCentralQueueFirstRetryUsesInitialDelay(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	endpoint := "endpoint-1:4317"
	p := &metricExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{endpoint}),
			exporters: map[string]*wrappedExporter{endpoint: newWrappedExporter(newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
				return status.Error(codes.Unavailable, "backend unavailable")
			}), endpoint)},
			endpointHealth: newEndpointHealthManager(endpointHealthSettings{}),
			res:            &mockResolver{},
		},
		telemetry: tb,
		logger:    ts.Logger,
	}

	ctx, cancel := context.WithCancel(t.Context())
	p.centralWG.Add(1)
	go p.runCentralQueue(ctx)
	item, err := newCentralQueueMetricsItem([]byte("route-a"), singleDataPointMetric("retry"), codec, time.Now())
	require.NoError(t, err)
	require.NoError(t, p.centralQueue.enqueue(item))

	requireCentralQueueFirstRetryDelay(t, p.centralQueue)
	cancel()
	require.NoError(t, waitForInflight(t.Context(), &p.centralWG))
}

func TestMetricsCentralQueueRetriesOnlyFailedEndpointItem(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	var callsMu sync.Mutex
	calls := map[string]int{}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
			callsMu.Lock()
			calls[endpoint]++
			callsMu.Unlock()
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	failedRoute := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	successRoute := findRoutingIDForEndpoint(t, lb.ring, "endpoint-2:4317")
	p := &metricExporterImp{
		centralQueue: newCentralQueue(centralQueueSettings{
			maxCompressedBytes:           1 << 20,
			maxInflightUncompressedBytes: 1 << 20,
			maxUncompressedBatchBytes:    1 << 20,
		}),
		centralCodec: codec,
		loadBalancer: lb,
		logger:       ts.Logger,
		telemetry:    tb,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	p.centralWG.Add(1)
	go p.runCentralQueue(ctx)

	failedItem, err := newCentralQueueMetricsItem([]byte(failedRoute), metricsWithServiceNames(failedRoute), codec, time.Now())
	require.NoError(t, err)
	// Keep the retry far enough out that slow CI cannot process it before assertions.
	failedItem.attempt = 10
	require.NoError(t, p.centralQueue.enqueue(failedItem))
	successItem, err := newCentralQueueMetricsItem([]byte(successRoute), metricsWithServiceNames(successRoute), codec, time.Now())
	require.NoError(t, err)
	require.NoError(t, p.centralQueue.enqueue(successItem))

	require.Eventually(t, func() bool {
		callsMu.Lock()
		defer callsMu.Unlock()
		return calls["endpoint-1:4317"] == 1 && calls["endpoint-2:4317"] == 1 && p.centralQueue.len() == 1
	}, time.Second, time.Millisecond)
	cancel()
	require.NoError(t, waitForInflight(t.Context(), &p.centralWG))

	p.centralQueue.mu.Lock()
	items := append([]centralQueueItem(nil), p.centralQueue.items...)
	p.centralQueue.mu.Unlock()
	require.Len(t, items, 1)
	remaining := items[0]
	require.Equal(t, []byte(failedRoute), remaining.routingKey)
	require.Equal(t, 11, remaining.attempt)
	require.NotZero(t, remaining.nextAttemptUnixNano)
}

func TestMetricsCentralQueueDoesNotDirectReroute(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)
	codec := newQueuePayloadCodec(QueuePayloadCompressionZstd)
	t.Cleanup(func() { require.NoError(t, codec.Close()) })

	calls := map[string]int{}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
			calls[endpoint]++
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	serviceForEndpoint1 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	item, err := newCentralQueueMetricsItem(
		[]byte(serviceForEndpoint1),
		metricsWithServiceNames(serviceForEndpoint1),
		codec,
		time.Now(),
	)
	require.NoError(t, err)

	p := &metricExporterImp{
		centralCodec: codec,
		loadBalancer: lb,
		logger:       ts.Logger,
		telemetry:    tb,
	}

	err = p.consumeCentralQueueMetricItem(t.Context(), item)
	require.Error(t, err)
	assert.Equal(t, 1, calls["endpoint-1:4317"])
	assert.Zero(t, calls["endpoint-2:4317"])
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

func cloneMetricsMap(src map[string]pmetric.Metrics) map[string]pmetric.Metrics {
	cloned := make(map[string]pmetric.Metrics, len(src))
	for key, md := range src {
		mdClone := pmetric.NewMetrics()
		md.CopyTo(mdClone)
		cloned[key] = mdClone
	}
	return cloned
}

func splitMetricsByMetricNameOld(md pmetric.Metrics) map[string]pmetric.Metrics {
	results := map[string]pmetric.Metrics{}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)

				newMD, mClone := cloneMetricWithoutType(rm, sm, m)
				m.CopyTo(mClone)

				key := m.Name()
				existing, ok := results[key]
				if ok {
					metrics.Merge(existing, newMD)
				} else {
					results[key] = newMD
				}
			}
		}
	}

	return results
}

func (e *metricExporterImp) groupRoutedMetricsByEndpointOld(
	batches map[string]pmetric.Metrics,
) (map[string]*endpointMetricsBatch, error) {
	endpointBatches := make(map[string]*endpointMetricsBatch)

	for routingID, mds := range batches {
		exp, endpoint, err := e.loadBalancer.exporterAndEndpoint([]byte(routingID))
		if err != nil {
			return nil, err
		}

		ep := endpointWithPort(endpoint)
		batch, ok := endpointBatches[ep]
		if !ok {
			batch = &endpointMetricsBatch{metrics: pmetric.NewMetrics(), exp: exp}
			endpointBatches[ep] = batch
		}
		batch.exp = exp
		metrics.Merge(batch.metrics, mds)
	}

	return endpointBatches, nil
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

func TestSplitMetricsByMetricNameMergesByResourceScopeAndName(t *testing.T) {
	t.Parallel()

	md1 := metricWithResourceScopeGauge("host-a", "scope-a", "m1", 1)
	md2 := metricWithResourceScopeGauge("host-a", "scope-a", "m1", 2)
	combined := pmetric.NewMetrics()
	md1.CopyTo(combined)
	mergeMetricChunksByMove(combined, md2)

	output := splitMetricsByMetricName(combined)
	require.Len(t, output, 1)

	split := output["m1"]
	require.Equal(t, 1, split.ResourceMetrics().Len())
	rm := split.ResourceMetrics().At(0)
	require.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)
	require.Equal(t, 1, sm.Metrics().Len())
	require.Equal(t, 2, sm.Metrics().At(0).Gauge().DataPoints().Len())
}

func TestSplitMetricsByMetricNameKeepsDistinctScopesSeparate(t *testing.T) {
	t.Parallel()

	md1 := metricWithResourceScopeGauge("host-a", "scope-a", "m1", 1)
	md2 := metricWithResourceScopeGauge("host-a", "scope-b", "m1", 2)
	combined := pmetric.NewMetrics()
	md1.CopyTo(combined)
	mergeMetricChunksByMove(combined, md2)

	output := splitMetricsByMetricName(combined)
	require.Len(t, output, 1)

	split := output["m1"]
	require.Equal(t, 1, split.ResourceMetrics().Len())
	require.Equal(t, 2, split.ResourceMetrics().At(0).ScopeMetrics().Len())
}

func TestSplitMetricsByMetricNameMatchesOldImplementation(t *testing.T) {
	t.Parallel()

	base := multiTypeMetricNameFixture()
	oldInput := pmetric.NewMetrics()
	newInput := pmetric.NewMetrics()
	base.CopyTo(oldInput)
	base.CopyTo(newInput)

	expected := splitMetricsByMetricNameOld(oldInput)
	actual := splitMetricsByMetricName(newInput)

	compareMetricsMaps(t, expected, actual)
}

func TestGroupRoutedMetricsByEndpointMergesRoutedMetricsForSameEndpoint(t *testing.T) {
	t.Parallel()

	p := &metricExporterImp{
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{"endpoint-1"}),
			exporters: map[string]*wrappedExporter{
				"endpoint-1:4317": newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317"),
			},
		},
	}

	batches := map[string]pmetric.Metrics{
		"route-a": singleDataPointMetric("m1"),
		"route-b": singleDataPointMetric("m2"),
	}
	grouped, err := p.groupRoutedMetricsByEndpoint(batches)
	require.NoError(t, err)
	require.Len(t, grouped, 1)

	batch := grouped["endpoint-1:4317"]
	require.NotNil(t, batch)
	require.Equal(t, 2, batch.metrics.DataPointCount())
	nameSplit := splitMetricsByMetricNameOld(batch.metrics)
	require.Len(t, nameSplit, 2)
	_, hasM1 := nameSplit["m1"]
	_, hasM2 := nameSplit["m2"]
	require.True(t, hasM1)
	require.True(t, hasM2)
}

func TestGroupRoutedMetricsByEndpointReturnsPartialBatchesOnLookupError(t *testing.T) {
	t.Parallel()

	ring := newHashRing([]string{"endpoint-1", "endpoint-2"})
	okRoute := findRoutingIDForEndpoint(t, ring, "endpoint-1")
	missingRoute := findRoutingIDForEndpoint(t, ring, "endpoint-2")

	p := &metricExporterImp{
		loadBalancer: &loadBalancer{
			ring: ring,
			exporters: map[string]*wrappedExporter{
				"endpoint-1:4317": newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317"),
			},
		},
	}

	batches := map[string]pmetric.Metrics{
		okRoute:      singleDataPointMetric(okRoute),
		missingRoute: singleDataPointMetric(missingRoute),
	}
	expectedFailed := pmetric.NewMetrics()
	for _, md := range batches {
		metrics.Merge(expectedFailed, md)
	}

	grouped, err := p.groupRoutedMetricsByEndpoint(batches)
	require.Error(t, err)

	failed := pmetric.NewMetrics()
	for _, batch := range grouped {
		metrics.Merge(failed, batch.metrics)
	}
	for _, md := range batches {
		metrics.Merge(failed, md)
	}
	require.NoError(t, pmetrictest.CompareMetrics(
		expectedFailed, failed,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestGroupRoutedMetricsByEndpointMatchesOldImplementation(t *testing.T) {
	t.Parallel()

	imp := &metricExporterImp{
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{"endpoint-1"}),
			exporters: map[string]*wrappedExporter{
				"endpoint-1:4317": newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317"),
			},
		},
	}
	base := map[string]pmetric.Metrics{
		"route-a": metricWithResourceScopeGauge("host-a", "scope-a", "cpu", 1),
		"route-b": metricWithResourceScopeGauge("host-a", "scope-a", "cpu", 2),
		"route-c": metricWithResourceScopeGauge("host-b", "scope-b", "memory", 3),
	}

	expected, err := imp.groupRoutedMetricsByEndpointOld(cloneMetricsMap(base))
	require.NoError(t, err)

	actual, err := imp.groupRoutedMetricsByEndpoint(cloneMetricsMap(base))
	require.NoError(t, err)

	require.Len(t, actual, len(expected))
	for endpoint, expectedBatch := range expected {
		actualBatch, ok := actual[endpoint]
		require.True(t, ok)
		require.Same(t, expectedBatch.exp, actualBatch.exp)
		require.Equal(t, gaugeDataPointValuesByMetricName(expectedBatch.metrics), gaugeDataPointValuesByMetricName(actualBatch.metrics))
	}
}

func TestRerouteDrainBatchReturnsAllMetricsOnLookupErrorAfterMoveSplit(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	lb := &loadBalancer{ring: newHashRing([]string{"endpoint-1", "endpoint-2"})}
	okRoute := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1")
	missingRoute := findRoutingIDForEndpoint(t, lb.ring, "endpoint-2")
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317"),
	}

	p := &metricExporterImp{
		routingKey:   metricNameRouting,
		loadBalancer: lb,
		telemetry:    tb,
		logger:       ts.Logger,
	}

	input := twoMetricRoutingInput(okRoute, missingRoute)
	expectedFailed := pmetric.NewMetrics()
	input.CopyTo(expectedFailed)

	err := p.rerouteDrainBatch(t.Context(), input, metricFlushReasonResolverChange)
	require.Error(t, err)

	var mErr consumererror.Metrics
	require.ErrorAs(t, err, &mErr)
	require.NoError(t, pmetrictest.CompareMetrics(
		expectedFailed, mErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestRerouteDrainBatchReturnsAllMetricsWhenExporterStoppingAfterMoveSplit(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	endpointA := "endpoint-a:4317"
	endpointB := "endpoint-b:4317"
	lb := &loadBalancer{ring: newHashRing([]string{endpointA, endpointB})}

	routingA := findRoutingIDForEndpoint(t, lb.ring, endpointA)
	routingB := findRoutingIDForEndpoint(t, lb.ring, endpointB)

	openEndpoint, openRouting := endpointA, routingA
	stoppingEndpoint, stoppingRouting := endpointB, routingB
	if routingA > routingB {
		openEndpoint, openRouting = endpointB, routingB
		stoppingEndpoint, stoppingRouting = endpointA, routingA
	}

	openExporter := newWrappedExporter(newNopMockMetricsExporter(), openEndpoint)
	stoppingExporter := newWrappedExporter(newNopMockMetricsExporter(), stoppingEndpoint)
	stoppingExporter.markStopping()

	lb.exporters = map[string]*wrappedExporter{
		openEndpoint:     openExporter,
		stoppingEndpoint: stoppingExporter,
	}

	p := &metricExporterImp{
		routingKey:   metricNameRouting,
		loadBalancer: lb,
		telemetry:    tb,
		logger:       ts.Logger,
	}

	input := twoMetricRoutingInput(openRouting, stoppingRouting)
	expectedFailed := pmetric.NewMetrics()
	input.CopyTo(expectedFailed)

	err := p.rerouteDrainBatch(t.Context(), input, metricFlushReasonResolverChange)
	require.Error(t, err)

	var mErr consumererror.Metrics
	require.ErrorAs(t, err, &mErr)
	require.NoError(t, pmetrictest.CompareMetrics(
		expectedFailed, mErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
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

func TestConsumeMetricsReroutesEndpointLocalFailure(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)

	calls := map[string]int{}
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
			calls[endpoint]++
			if endpoint == "endpoint-1:4317" {
				return context.DeadlineExceeded
			}
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb
	p.started.Store(true)

	serviceForEndpoint1 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	err = p.ConsumeMetrics(t.Context(), metricsWithServiceNames(serviceForEndpoint1))
	require.NoError(t, err)

	assert.Equal(t, 1, calls["endpoint-1:4317"])
	assert.Equal(t, 1, calls["endpoint-2:4317"])
	assert.NotContains(t, lb.exporters, "endpoint-1:4317")
	metadatatest.AssertEqualLoadbalancerBackendQuarantineTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("endpoint", "endpoint-1:4317"), attribute.String("reason", "timeout")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
	metadatatest.AssertEqualLoadbalancerBackendRerouteTotal(t, telemetry, []metricdata.DataPoint[int64]{
		{
			Attributes: attribute.NewSet(attribute.String("signal", "metrics"), attribute.String("result", "success"), attribute.String("reason", "timeout")),
			Value:      1,
		},
	}, metricdatatest.IgnoreTimestamp())
}

func TestConsumeMetricsRecordsBackendRequestMetrics(t *testing.T) {
	ts, tb, telemetry := getTelemetryAssetsWithReader(t)
	cfg := endpoint2Config()
	routeEndpoint := "endpoint-2"
	backendEndpoint := "endpoint-2:4317"

	sent := pmetric.NewMetrics()
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, md pmetric.Metrics) error {
			if endpoint == backendEndpoint {
				md.CopyTo(sent)
			}
			return nil
		}), nil
	}

	p, lb := newTestMetricsExporter(t, ts, tb, cfg, componentFactory)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	serviceForEndpoint2 := findRoutingIDForEndpoint(t, lb.ring, routeEndpoint)
	require.NoError(t, p.ConsumeMetrics(t.Context(), metricsWithServiceDataPoints(serviceForEndpoint2, serviceForEndpoint2)))

	assert.Equal(t, 2, sent.DataPointCount())
	assertBackendRequestMetrics(
		t,
		telemetry,
		backendRequestSignalMetrics,
		backendEndpoint,
		serializedMetricsSize(sent),
		int64(sent.DataPointCount()),
	)
}

func TestConsumeMetricsCentralByteBatchPartialFailureReturnsOnlyFailedEndpointSubset(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := serviceBasedRoutingConfig()
	enableCentralQueueByteBatchingForTest(cfg)

	ring := newHashRing([]string{"endpoint-1", "endpoint-2"})
	serviceForEndpoint1 := findRoutingIDForEndpoint(t, ring, "endpoint-1")
	serviceForEndpoint2 := findRoutingIDForEndpoint(t, ring, "endpoint-2")

	var endpoint1Records int
	var endpoint2Records int
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, md pmetric.Metrics) error {
			switch endpoint {
			case "endpoint-1:4317":
				endpoint1Records += md.DataPointCount()
				return status.Error(codes.Unavailable, "backend unavailable")
			case "endpoint-2:4317":
				endpoint2Records += md.DataPointCount()
			}
			return nil
		}), nil
	}

	p, _ := newTestMetricsExporter(t, ts, tb, cfg, componentFactory)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	input := metricsWithServiceDataPoints(serviceForEndpoint1, serviceForEndpoint2, serviceForEndpoint2)
	err := p.ConsumeMetrics(t.Context(), input)
	require.Error(t, err)

	assert.Equal(t, 1, endpoint1Records)
	assert.Equal(t, 2, endpoint2Records)

	var metricsErr consumererror.Metrics
	require.ErrorAs(t, err, &metricsErr)
	require.NoError(t, pmetrictest.CompareMetrics(
		metricsWithServiceDataPoints(serviceForEndpoint1),
		metricsErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func BenchmarkConsumeMetricsRouteAwareMergedInput(b *testing.B) {
	ring := newHashRing([]string{"endpoint-1", "endpoint-2"})
	serviceForEndpoint1 := findRoutingIDForEndpoint(b, ring, "endpoint-1")
	serviceForEndpoint2 := findRoutingIDForEndpoint(b, ring, "endpoint-2")
	input := metricsWithServiceDataPoints(
		serviceForEndpoint1, serviceForEndpoint1, serviceForEndpoint1, serviceForEndpoint1,
		serviceForEndpoint2, serviceForEndpoint2, serviceForEndpoint2, serviceForEndpoint2,
	)
	serializedBytes := serializedMetricsSize(input)

	ts, tb := getTelemetryAssets(b)
	cfg := serviceBasedRoutingConfig()
	enableCentralQueueByteBatchingForTest(cfg)

	var sendCount atomic.Int64
	componentFactory := func(_ context.Context, _ string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, _ pmetric.Metrics) error {
			sendCount.Add(1)
			return nil
		}), nil
	}

	p, _ := newTestMetricsExporter(b, ts, tb, cfg, componentFactory)
	require.NoError(b, p.Start(b.Context(), componenttest.NewNopHost()))
	b.Cleanup(func() {
		require.NoError(b, p.Shutdown(context.WithoutCancel(b.Context())))
	})

	b.SetBytes(serializedBytes)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		require.NoError(b, p.ConsumeMetrics(b.Context(), input))
	}
	b.StopTimer()

	requestsPerInput := float64(sendCount.Load()) / float64(b.N)
	b.ReportMetric(requestsPerInput, "backend_requests/input")
	require.Equal(b, float64(2), requestsPerInput)
}

func TestConsumeMetricsReroutePreservesPayloadAfterExporterMutation(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)

	var rerouted pmetric.Metrics
	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, md pmetric.Metrics) error {
			if endpoint == "endpoint-1:4317" {
				md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("mutated")
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			rerouted = pmetric.NewMetrics()
			md.CopyTo(rerouted)
			return nil
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb
	p.started.Store(true)

	serviceForEndpoint1 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	input := metricsWithServiceNames(serviceForEndpoint1)
	expected := metricsWithServiceNames(serviceForEndpoint1)
	require.NoError(t, p.ConsumeMetrics(t.Context(), input))

	require.NoError(t, pmetrictest.CompareMetrics(
		expected,
		rerouted,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestConsumeMetricsRerouteFailureReturnsConsumerErrorMetrics(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)

	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return errors.New("reroute failed")
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb
	p.started.Store(true)

	serviceForEndpoint1 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	input := metricsWithServiceNames(serviceForEndpoint1)
	err = p.ConsumeMetrics(t.Context(), input)
	require.Error(t, err)

	var metricsErr consumererror.Metrics
	require.ErrorAs(t, err, &metricsErr)
	require.NoError(t, pmetrictest.CompareMetrics(
		metricsWithServiceNames(serviceForEndpoint1),
		metricsErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestConsumeMetricsRerouteFailurePreservesConsumerErrorMetricsAfterExporterMutation(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)

	componentFactory := func(_ context.Context, endpoint string) (component.Component, error) {
		return newMockMetricsExporter(func(_ context.Context, md pmetric.Metrics) error {
			md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).SetName("mutated")
			if endpoint == "endpoint-1:4317" {
				return status.Error(codes.Unavailable, "backend unavailable")
			}
			return errors.New("reroute failed")
		}), nil
	}
	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.addMissingExporters(t.Context(), []string{"endpoint-1:4317", "endpoint-2:4317"})

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb
	p.started.Store(true)

	serviceForEndpoint1 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1:4317")
	input := metricsWithServiceNames(serviceForEndpoint1)
	expected := metricsWithServiceNames(serviceForEndpoint1)
	err = p.ConsumeMetrics(t.Context(), input)
	require.Error(t, err)

	var metricsErr consumererror.Metrics
	require.ErrorAs(t, err, &metricsErr)
	require.NoError(t, pmetrictest.CompareMetrics(
		expected,
		metricsErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestConsumeMetricsBatcherFlushDoesNotQuarantineEndpoint(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	cfg := endpoint2Config()
	enableEndpointHealth(cfg)

	lb, err := newLoadBalancer(ts.Logger, cfg, func(context.Context, string) (component.Component, error) {
		return newNopMockMetricsExporter(), nil
	}, tb)
	require.NoError(t, err)
	lb.endpointHealth.reconcile([]string{"endpoint-1:4317", "endpoint-2:4317"})
	lb.ring = newHashRing([]string{"endpoint-1:4317", "endpoint-2:4317"})
	exp := newWrappedExporter(newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
		return status.Error(codes.Unavailable, "backend unavailable")
	}), "endpoint-1:4317")
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": exp,
	}

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb

	err = p.consumeBatch(t.Context(), exp, simpleMetricsWithServiceName(), metricFlushReasonShutdown)
	require.Error(t, err)
	assert.Contains(t, lb.exporters, "endpoint-1:4317")
	assert.ElementsMatch(t, []string{"endpoint-1:4317", "endpoint-2:4317"}, lb.endpointHealth.eligibleEndpoints())
}

func TestConsumeMetrics_SingleEndpointWithMetricBatcher(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	config := &Config{
		Resolver: ResolverSettings{
			Static: configoptional.Some(StaticResolver{Hostnames: []string{"endpoint-1"}}),
		},
		RoutingKey: metricNameRoutingStr,
		MetricBatcher: MetricBatcherConfig{
			Enabled:       true,
			MaxDataPoints: 2,
			MaxBytes:      1 << 20,
			FlushInterval: time.Hour,
		},
	}

	p, err := newMetricsExporter(ts, config)
	require.NoError(t, err)
	require.NotNil(t, p)

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

	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	input1 := singleDataPointMetric("batcher-metric-1")
	input1.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, serviceName1)
	err = p.ConsumeMetrics(t.Context(), input1)
	require.NoError(t, err)
	require.Empty(t, sink.AllMetrics(), "first call should stay buffered in metric batcher")

	input2 := singleDataPointMetric("batcher-metric-2")
	input2.ResourceMetrics().At(0).Resource().Attributes().PutStr(serviceNameKey, serviceName1)
	err = p.ConsumeMetrics(t.Context(), input2)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(sink.AllMetrics()) == 1
	}, time.Second, 10*time.Millisecond)

	allOutputs := sink.AllMetrics()
	require.Len(t, allOutputs, 1)
	actual := allOutputs[0]
	require.Equal(t, 2, actual.DataPointCount())

	names := map[string]struct{}{}
	resourceMetrics := actual.ResourceMetrics()
	for i := range resourceMetrics.Len() {
		scopeMetrics := resourceMetrics.At(i).ScopeMetrics()
		for j := range scopeMetrics.Len() {
			metrics := scopeMetrics.At(j).Metrics()
			for k := range metrics.Len() {
				names[metrics.At(k).Name()] = struct{}{}
			}
		}
	}

	_, hasMetric1 := names["batcher-metric-1"]
	_, hasMetric2 := names["batcher-metric-2"]
	require.True(t, hasMetric1)
	require.True(t, hasMetric2)
}

func TestEnqueueEndpointBatchesMakesProgressWhenOneQueueIsFull(t *testing.T) {
	fullBackend := &backendMetricBatcher{requests: make(chan metricBatcherRequest, 1), done: make(chan struct{})}
	openBackend := &backendMetricBatcher{requests: make(chan metricBatcherRequest, 1), done: make(chan struct{})}
	fullBackend.requests <- metricBatcherRequest{kind: metricBatcherRequestEnqueue, md: singleDataPointMetric("prefilled")}

	p := &metricExporterImp{
		batcher: &metricBatcher{
			backends: map[string]*backendMetricBatcher{
				"full:4317": fullBackend,
				"open:4317": openBackend,
			},
		},
	}

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err := p.enqueueEndpointBatches(ctx, map[string]*endpointMetricsBatch{
		"full:4317": {
			metrics: singleDataPointMetric("m1"),
			exp:     newWrappedExporter(newNopMockMetricsExporter(), "full:4317"),
		},
		"open:4317": {
			metrics: singleDataPointMetric("m2"),
			exp:     newWrappedExporter(newNopMockMetricsExporter(), "open:4317"),
		},
	}, false)

	var mErr consumererror.Metrics
	require.ErrorAs(t, err, &mErr)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Len(t, openBackend.requests, 1)
	require.NoError(t, pmetrictest.CompareMetrics(
		singleDataPointMetric("m1"),
		mErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestEnqueueEndpointBatchesReturnsFailedSubsetOnBackendError(t *testing.T) {
	stoppedBackend := &backendMetricBatcher{requests: make(chan metricBatcherRequest, 1), done: make(chan struct{})}
	close(stoppedBackend.done)
	openBackend := &backendMetricBatcher{requests: make(chan metricBatcherRequest, 1), done: make(chan struct{})}

	p := &metricExporterImp{
		batcher: &metricBatcher{
			backends: map[string]*backendMetricBatcher{
				"stopped:4317": stoppedBackend,
				"open:4317":    openBackend,
			},
		},
	}

	err := p.enqueueEndpointBatches(t.Context(), map[string]*endpointMetricsBatch{
		"stopped:4317": {
			metrics: singleDataPointMetric("m1"),
			exp:     newWrappedExporter(newNopMockMetricsExporter(), "stopped:4317"),
		},
		"open:4317": {
			metrics: singleDataPointMetric("m2"),
			exp:     newWrappedExporter(newNopMockMetricsExporter(), "open:4317"),
		},
	}, false)

	var mErr consumererror.Metrics
	require.ErrorAs(t, err, &mErr)
	require.ErrorIs(t, err, errMetricBatcherExporterStopping)
	require.Len(t, openBackend.requests, 1)
	require.NoError(t, pmetrictest.CompareMetrics(
		singleDataPointMetric("m1"),
		mErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestEnqueueEndpointBatchesReroutesOnExporterStopping(t *testing.T) {
	stoppedBackend := &backendMetricBatcher{requests: make(chan metricBatcherRequest, 1), done: make(chan struct{})}
	openBackend := &backendMetricBatcher{requests: make(chan metricBatcherRequest, 1), done: make(chan struct{})}

	openExporter := newWrappedExporter(newNopMockMetricsExporter(), "open:4317")

	p := &metricExporterImp{
		routingKey: metricNameRouting,
		loadBalancer: &loadBalancer{
			ring: newHashRing([]string{"open:4317"}),
			exporters: map[string]*wrappedExporter{
				"open:4317": openExporter,
			},
		},
		batcher: &metricBatcher{
			backends: map[string]*backendMetricBatcher{
				"stopped:4317": stoppedBackend,
				"open:4317":    openBackend,
			},
		},
	}

	stoppingExporter := newWrappedExporter(newNopMockMetricsExporter(), "stopped:4317")
	stoppingExporter.markStopping()

	err := p.enqueueEndpointBatches(t.Context(), map[string]*endpointMetricsBatch{
		"stopped:4317": {
			metrics: singleDataPointMetric("reroute-metric"),
			exp:     stoppingExporter,
		},
	}, true)
	require.NoError(t, err)
	require.Empty(t, stoppedBackend.requests)

	require.Len(t, openBackend.requests, 1)
	request := <-openBackend.requests
	require.Equal(t, metricBatcherRequestEnqueue, request.kind)
	require.NoError(t, pmetrictest.CompareMetrics(
		singleDataPointMetric("reroute-metric"),
		request.md,
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestEnqueueEndpointBatchesReturnsOriginalMetricsOnPartialRegroupError(t *testing.T) {
	t.Parallel()

	ring := newHashRing([]string{"endpoint-1", "endpoint-2"})
	okRoute := findRoutingIDForEndpoint(t, ring, "endpoint-1")
	missingRoute := findRoutingIDForEndpoint(t, ring, "endpoint-2")

	stoppingExporter := newWrappedExporter(newNopMockMetricsExporter(), "stopped:4317")
	stoppingExporter.markStopping()

	p := &metricExporterImp{
		routingKey: metricNameRouting,
		loadBalancer: &loadBalancer{
			ring: ring,
			exporters: map[string]*wrappedExporter{
				"endpoint-1:4317": newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317"),
			},
		},
		batcher: &metricBatcher{backends: map[string]*backendMetricBatcher{}},
	}

	input := twoMetricRoutingInput(okRoute, missingRoute)
	expected := pmetric.NewMetrics()
	input.CopyTo(expected)

	err := p.enqueueEndpointBatches(t.Context(), map[string]*endpointMetricsBatch{
		"stopped:4317": {
			metrics: input,
			exp:     stoppingExporter,
		},
	}, true)
	require.Error(t, err)

	var mErr consumererror.Metrics
	require.ErrorAs(t, err, &mErr)
	require.NoError(t, pmetrictest.CompareMetrics(
		expected, mErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestRerouteDrainBatchReturnsFailedSubsetOnPartialSuccess(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	lb := &loadBalancer{ring: newHashRing([]string{"ok", "fail"})}
	metricForOK := findRoutingIDForEndpoint(t, lb.ring, "ok")
	metricForFail := findRoutingIDForEndpoint(t, lb.ring, "fail")

	consumed := make(chan int, 1)
	okExporter := newWrappedExporter(newMockMetricsExporter(func(_ context.Context, md pmetric.Metrics) error {
		consumed <- md.DataPointCount()
		return nil
	}), "ok:4317")
	failExporter := newWrappedExporter(newMockMetricsExporter(func(context.Context, pmetric.Metrics) error {
		return errors.New("backend failed")
	}), "fail:4317")

	lb.exporters = map[string]*wrappedExporter{
		"ok:4317":   okExporter,
		"fail:4317": failExporter,
	}

	p := &metricExporterImp{
		routingKey:   metricNameRouting,
		loadBalancer: lb,
		telemetry:    tb,
		logger:       ts.Logger,
	}

	input := pmetric.NewMetrics()
	rm := input.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName(metricForOK)
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName(metricForFail)
	m2.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

	err := p.rerouteDrainBatch(t.Context(), input, metricFlushReasonResolverChange)
	require.Error(t, err)

	var mErr consumererror.Metrics
	require.ErrorAs(t, err, &mErr)

	select {
	case got := <-consumed:
		require.Equal(t, 1, got)
	case <-time.After(2 * time.Second):
		t.Fatal("expected successful reroute send")
	}

	require.NoError(t, pmetrictest.CompareMetrics(
		singleDataPointMetric(metricForFail),
		mErr.Data(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreScopeMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func twoMetricRoutingInput(firstName, secondName string) pmetric.Metrics {
	input := pmetric.NewMetrics()
	rm := input.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m1 := sm.Metrics().AppendEmpty()
	m1.SetName(firstName)
	m1.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	m2 := sm.Metrics().AppendEmpty()
	m2.SetName(secondName)
	m2.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2)
	return input
}

func multiTypeMetricNameFixture() pmetric.Metrics {
	md := pmetric.NewMetrics()

	rmA := md.ResourceMetrics().AppendEmpty()
	rmA.Resource().Attributes().PutStr("host.name", "host-a")
	smA1 := rmA.ScopeMetrics().AppendEmpty()
	smA1.Scope().SetName("scope-a")

	gaugeA1 := smA1.Metrics().AppendEmpty()
	gaugeA1.SetName("cpu")
	gaugeA1.SetDescription("cpu gauge")
	gaugeA1.SetUnit("1")
	gaugeA1.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

	gaugeA2 := smA1.Metrics().AppendEmpty()
	gaugeA2.SetName("cpu")
	gaugeA2.SetDescription("cpu gauge")
	gaugeA2.SetUnit("1")
	gaugeA2.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(2)

	sumA := smA1.Metrics().AppendEmpty()
	sumA.SetName("requests")
	sumA.SetUnit("1")
	sumA.SetDescription("request count")
	sumData := sumA.SetEmptySum()
	sumData.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	sumData.SetIsMonotonic(true)
	sumData.DataPoints().AppendEmpty().SetIntValue(3)

	histA := smA1.Metrics().AppendEmpty()
	histA.SetName("latency")
	histA.SetUnit("ms")
	histA.SetDescription("latency histogram")
	histData := histA.SetEmptyHistogram()
	histData.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	histDP := histData.DataPoints().AppendEmpty()
	histDP.SetCount(2)
	histDP.SetSum(9)
	histDP.BucketCounts().FromRaw([]uint64{1, 1})
	histDP.ExplicitBounds().FromRaw([]float64{5})

	smA2 := rmA.ScopeMetrics().AppendEmpty()
	smA2.Scope().SetName("scope-b")

	expHist := smA2.Metrics().AppendEmpty()
	expHist.SetName("exp-latency")
	expHist.SetUnit("ms")
	expHist.SetDescription("exp histogram")
	expHistData := expHist.SetEmptyExponentialHistogram()
	expHistData.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	expHistDP := expHistData.DataPoints().AppendEmpty()
	expHistDP.SetCount(1)
	expHistDP.SetSum(4)
	expHistDP.Positive().SetOffset(0)
	expHistDP.Positive().BucketCounts().FromRaw([]uint64{1})

	rmB := md.ResourceMetrics().AppendEmpty()
	rmB.Resource().Attributes().PutStr("host.name", "host-b")
	smB := rmB.ScopeMetrics().AppendEmpty()
	smB.Scope().SetName("scope-a")

	summary := smB.Metrics().AppendEmpty()
	summary.SetName("summary")
	summary.SetUnit("1")
	summary.SetDescription("summary metric")
	summaryDP := summary.SetEmptySummary().DataPoints().AppendEmpty()
	summaryDP.SetCount(1)
	summaryDP.SetSum(7)
	summaryDP.QuantileValues().AppendEmpty().SetQuantile(0.5)
	summaryDP.QuantileValues().At(0).SetValue(7)

	return md
}

func gaugeDataPointValuesByMetricName(md pmetric.Metrics) map[string][]int64 {
	valuesByMetric := make(map[string][]int64)
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				values := valuesByMetric[metric.Name()]
				for l := 0; l < metric.Gauge().DataPoints().Len(); l++ {
					values = append(values, metric.Gauge().DataPoints().At(l).IntValue())
				}
				valuesByMetric[metric.Name()] = values
			}
		}
	}
	for name := range valuesByMetric {
		slices.Sort(valuesByMetric[name])
	}
	return valuesByMetric
}

func TestRerouteDrainBatchReleasesStartedConsumesOnEarlyReturn(t *testing.T) {
	ts, tb := getTelemetryAssets(t)

	endpointA := "endpoint-a:4317"
	endpointB := "endpoint-b:4317"
	lb := &loadBalancer{ring: newHashRing([]string{endpointA, endpointB})}

	routingA := findRoutingIDForEndpoint(t, lb.ring, endpointA)
	routingB := findRoutingIDForEndpoint(t, lb.ring, endpointB)

	openEndpoint, openRouting := endpointA, routingA
	stoppingEndpoint, stoppingRouting := endpointB, routingB
	if routingA > routingB {
		openEndpoint, openRouting = endpointB, routingB
		stoppingEndpoint, stoppingRouting = endpointA, routingA
	}

	openExporter := newWrappedExporter(newNopMockMetricsExporter(), openEndpoint)
	stoppingExporter := newWrappedExporter(newNopMockMetricsExporter(), stoppingEndpoint)
	stoppingExporter.markStopping()

	lb.exporters = map[string]*wrappedExporter{
		openEndpoint:     openExporter,
		stoppingEndpoint: stoppingExporter,
	}

	p := &metricExporterImp{
		routingKey:   metricNameRouting,
		loadBalancer: lb,
		telemetry:    tb,
		logger:       ts.Logger,
	}

	input := pmetric.NewMetrics()
	rm := input.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	mOpen := sm.Metrics().AppendEmpty()
	mOpen.SetName(openRouting)
	mOpen.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	mStopping := sm.Metrics().AppendEmpty()
	mStopping.SetName(stoppingRouting)
	mStopping.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

	err := p.rerouteDrainBatch(t.Context(), input, metricFlushReasonResolverChange)
	require.Error(t, err)

	var mErr consumererror.Metrics
	require.ErrorAs(t, err, &mErr)

	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- openExporter.Shutdown(t.Context())
	}()

	select {
	case shutdown := <-shutdownErr:
		require.NoError(t, shutdown)
	case <-time.After(time.Second):
		t.Fatal("expected started consume slot to be released on reroute early return")
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
			serviceNameKey: fmt.Sprintf("service-%d", rand.IntN(512)),
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
	rmetrics.Resource().Attributes().PutStr(serviceNameKey, serviceName1)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func metricsWithServiceNames(serviceNames ...string) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(len(serviceNames))
	for _, serviceName := range serviceNames {
		rm := metrics.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr(serviceNameKey, serviceName)
		appendSimpleMetricWithID(rm, signal1Name)
	}
	return metrics
}

func metricsWithServiceDataPoints(serviceNames ...string) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(len(serviceNames))
	for i, serviceName := range serviceNames {
		rm := metrics.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr(serviceNameKey, serviceName)
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName(fmt.Sprintf("metric-%d", i))
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(int64(i + 1))
	}
	return metrics
}

func simpleMetricsWithResource() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(1)
	rmetrics := metrics.ResourceMetrics().AppendEmpty()
	rmetrics.Resource().Attributes().PutStr(serviceNameKey, serviceName1)
	rmetrics.Resource().Attributes().PutStr(keyAttr1, valueAttr1)
	rmetrics.Resource().Attributes().PutInt(keyAttr2, valueAttr2)
	rmetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(signal1Name)
	return metrics
}

func twoServicesWithSameMetricName() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().EnsureCapacity(2)
	rs1 := metrics.ResourceMetrics().AppendEmpty()
	rs1.Resource().Attributes().PutStr(serviceNameKey, serviceName1)
	appendSimpleMetricWithID(rs1, signal1Name)
	rs2 := metrics.ResourceMetrics().AppendEmpty()
	rs2.Resource().Attributes().PutStr(serviceNameKey, serviceName2)
	appendSimpleMetricWithID(rs2, signal1Name)
	return metrics
}

func appendSimpleMetricWithID(dest pmetric.ResourceMetrics, id string) {
	dest.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName(id)
}

func newTestMetricsExporter(
	t testing.TB,
	ts exporter.Settings,
	tb *metadata.TelemetryBuilder,
	cfg *Config,
	componentFactory func(context.Context, string) (component.Component, error),
) (*metricExporterImp, *loadBalancer) {
	t.Helper()

	lb, err := newLoadBalancer(ts.Logger, cfg, componentFactory, tb)
	require.NoError(t, err)
	lb.addMissingExporters(context.Background(), cfg.Resolver.Static.Get().Hostnames)
	lb.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(_ context.Context) ([]string, error) {
			return cfg.Resolver.Static.Get().Hostnames, nil
		},
	}

	p, err := newMetricsExporter(ts, cfg)
	require.NoError(t, err)
	p.loadBalancer = lb
	return p, lb
}

func TestConsumeMetricsReleasesStartedConsumesOnEarlyReturn(t *testing.T) {
	ts, tb := getTelemetryAssets(t)
	lb, err := newLoadBalancer(ts.Logger, endpoint2Config(), nil, tb)
	require.NoError(t, err)

	lb.ring = newHashRing([]string{"endpoint-1", "endpoint-2"})

	serviceForEndpoint1 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-1")
	serviceForEndpoint2 := findRoutingIDForEndpoint(t, lb.ring, "endpoint-2")

	exp1 := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-1:4317")
	exp2 := newWrappedExporter(newNopMockMetricsExporter(), "endpoint-2:4317")
	exp2.markStopping()
	lb.exporters = map[string]*wrappedExporter{
		"endpoint-1:4317": exp1,
		"endpoint-2:4317": exp2,
	}

	p, err := newMetricsExporter(ts, endpoint2Config())
	require.NoError(t, err)
	p.loadBalancer = lb

	err = p.ConsumeMetrics(t.Context(), metricsWithServiceNames(serviceForEndpoint1, serviceForEndpoint2))
	require.ErrorIs(t, err, errExporterIsStopping)

	shutdownErr := make(chan error, 1)
	go func() {
		shutdownErr <- exp1.Shutdown(t.Context())
	}()

	select {
	case err := <-shutdownErr:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("expected started consume slot to be released on early return")
	}
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
