// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestBackgroundCleanup(t *testing.T) {
	// 1. Configure very short expiration (10ms)
	expiration := 10 * time.Millisecond
	cfg := &Config{MetricExpiration: expiration}

	c := newCollector(cfg, zap.NewNop())

	// FIX: Start the background loop manually!
	c.Start()
	defer c.stop()

	// 2. Inject a STALE metric (1 hour old) -> Should be removed
	staleMetricName := "stale_metric"
	c.metricFamilies.Store(staleMetricName, metricFamily{
		lastSeen: time.Now().Add(-1 * time.Hour),
	})

	// 3. Inject an ACTIVE metric (Future time) -> Should stay
	activeMetricName := "active_metric"
	c.metricFamilies.Store(activeMetricName, metricFamily{
		lastSeen: time.Now().Add(1 * time.Hour),
	})

	// 4. Wait for the loop to run (Wait longer to be safe)
	time.Sleep(100 * time.Millisecond)

	// 5. Assertions
	_, staleExists := c.metricFamilies.Load(staleMetricName)
	assert.False(t, staleExists, "FAILURE: The stale metric was NOT removed.")

	_, activeExists := c.metricFamilies.Load(activeMetricName)
	assert.True(t, activeExists, "FAILURE: The active metric was removed incorrectly.")
}
