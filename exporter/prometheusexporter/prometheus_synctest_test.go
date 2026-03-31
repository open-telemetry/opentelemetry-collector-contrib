// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build goexperiment.synctest

package prometheusexporter

import (
	"testing"
	"testing/synctest"
	"time"

	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// TestPrometheusExporter_BackgroundCleanup verifies that expired entries in both
// the accumulator's registeredMetrics and the collector's metricFamilies map are
// evicted by the background cleanup goroutine even when no Prometheus scrape occurs.
// Uses synctest for deterministic fake-clock testing instead of require.Eventually.
// Covers https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/41123
func TestPrometheusExporter_BackgroundCleanup(t *testing.T) {
	synctest.Run(func() {
		expiration := 5 * time.Minute
		c := newCollector(&Config{MetricExpiration: expiration}, zap.NewNop())
		a := c.accumulator.(*lastValueAccumulator)

		// Seed the accumulator with a stale and a fresh time series.
		staleMetric := pmetric.NewMetric()
		staleMetric.SetName("stale_accumulated")
		a.registeredMetrics.Store("stale_acc_key", &accumulatedValue{
			value:           staleMetric,
			resourceAttrs:   pcommon.NewMap(),
			scopeAttributes: pcommon.NewMap(),
			updated:         time.Now().Add(-10 * time.Minute),
		})
		freshMetric := pmetric.NewMetric()
		freshMetric.SetName("fresh_accumulated")
		a.registeredMetrics.Store("fresh_acc_key", &accumulatedValue{
			value:           freshMetric,
			resourceAttrs:   pcommon.NewMap(),
			scopeAttributes: pcommon.NewMap(),
			updated:         time.Now().Add(time.Hour),
		})

		// Seed the metricFamilies map with a stale and a fresh entry.
		gaugeType := io_prometheus_client.MetricType_GAUGE
		c.metricFamilies.Store("stale_metric", metricFamily{
			lastSeen: time.Now().Add(-10 * time.Minute),
			mf: &io_prometheus_client.MetricFamily{
				Name: proto.String("stale_metric"),
				Help: proto.String("should be cleaned up"),
				Type: &gaugeType,
			},
		})
		c.metricFamilies.Store("fresh_metric", metricFamily{
			lastSeen: time.Now().Add(time.Hour),
			mf: &io_prometheus_client.MetricFamily{
				Name: proto.String("fresh_metric"),
				Help: proto.String("should remain"),
				Type: &gaugeType,
			},
		})

		// Start the background cleanup goroutine (same logic as prometheusExporter.Start).
		stopCh := make(chan struct{})
		go func() {
			ticker := time.NewTicker(expiration)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					a.cleanupExpired()
					c.cleanupMetricFamilies()
				case <-stopCh:
					return
				}
			}
		}()
		defer close(stopCh)

		// Advance fake clock past the ticker interval so the cleanup fires.
		time.Sleep(expiration + time.Second)
		synctest.Wait()

		// Stale entries must have been evicted.
		_, mfFound := c.metricFamilies.Load("stale_metric")
		assert.False(t, mfFound, "stale_metric should have been evicted")
		_, accFound := a.registeredMetrics.Load("stale_acc_key")
		assert.False(t, accFound, "stale_accumulated should have been evicted")

		// Fresh entries must survive.
		_, ok := c.metricFamilies.Load("fresh_metric")
		assert.True(t, ok, "fresh_metric should not have been evicted")
		_, ok = a.registeredMetrics.Load("fresh_acc_key")
		assert.True(t, ok, "fresh_accumulated should not have been evicted")
	})
}
