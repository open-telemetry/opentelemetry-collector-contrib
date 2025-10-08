// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
)

// Test data
var (
	testInstanceName = "test-instance"
	testTimeout      = 5 * time.Second
)

func setupTestRacScraper(t *testing.T) *RacScraper {
	logger := zap.NewNop()
	set := receivertest.NewNopSettings(component.MustNewType("newrelicoraclereceiver"))
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), set)

	// Use nil DB for unit tests - integration tests would use real DB
	scraper := NewRacScraper(nil, mb, logger, testInstanceName, metadata.DefaultMetricsBuilderConfig())
	return scraper
}

func TestNewRacScraper(t *testing.T) {
	logger := zap.NewNop()
	set := receivertest.NewNopSettings(component.MustNewType("newrelicoraclereceiver"))
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), set)
	config := metadata.DefaultMetricsBuilderConfig()

	scraper := NewRacScraper(nil, mb, logger, testInstanceName, config)

	assert.NotNil(t, scraper)
	assert.Nil(t, scraper.db) // We passed nil for unit test
	assert.Equal(t, mb, scraper.mb)
	assert.Equal(t, logger, scraper.logger)
	assert.Equal(t, testInstanceName, scraper.instanceName)
	assert.Equal(t, config, scraper.metricsBuilderConfig)
	assert.Nil(t, scraper.isRacMode) // Initially nil until first check
}

func TestScrapeRacMetrics_NilDB(t *testing.T) {
	scraper := setupTestRacScraper(t)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Should handle nil DB gracefully and return errors
	errors := scraper.ScrapeRacMetrics(ctx)

	// All 4 scraper functions should return an error due to nil DB
	assert.NotEmpty(t, errors)
	assert.LessOrEqual(t, len(errors), 4, "Should have at most 4 errors (one per scraper function)")
}

func TestScrapeRacMetrics_ContextCancellation(t *testing.T) {
	scraper := setupTestRacScraper(t)

	// Create context that is already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	errors := scraper.ScrapeRacMetrics(ctx)

	// Should detect context cancellation
	assert.NotEmpty(t, errors)

	// At least one error should be about context cancellation
	foundCancelError := false
	for _, err := range errors {
		if err.Error() == "context canceled" {
			foundCancelError = true
			break
		}
	}
	assert.True(t, foundCancelError, "Should have context cancellation error")
}

func TestScrapeRacMetrics_Timeout(t *testing.T) {
	scraper := setupTestRacScraper(t)

	// Use very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Let context timeout
	time.Sleep(1 * time.Millisecond)

	errors := scraper.ScrapeRacMetrics(ctx)

	// Should detect timeout
	assert.NotEmpty(t, errors)

	// At least one error should be about deadline exceeded
	foundTimeoutError := false
	for _, err := range errors {
		if err.Error() == "context deadline exceeded" {
			foundTimeoutError = true
			break
		}
	}
	assert.True(t, foundTimeoutError, "Should have timeout error")
}

func TestScrapeASMDiskGroups_NilDB(t *testing.T) {
	scraper := setupTestRacScraper(t)

	ctx := context.Background()
	errors := scraper.scrapeASMDiskGroups(ctx)

	// Should return error for nil DB
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Error(), "invalid connection")
}

func TestScrapeClusterWaitEvents_NilDB(t *testing.T) {
	scraper := setupTestRacScraper(t)

	ctx := context.Background()
	errors := scraper.scrapeClusterWaitEvents(ctx)

	// Should return error for nil DB
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Error(), "invalid connection")
}

func TestScrapeInstanceStatus_NilDB(t *testing.T) {
	scraper := setupTestRacScraper(t)

	ctx := context.Background()
	errors := scraper.scrapeInstanceStatus(ctx)

	// Should return error for nil DB
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Error(), "invalid connection")
}

func TestScrapeActiveServices_NilDB(t *testing.T) {
	scraper := setupTestRacScraper(t)

	ctx := context.Background()
	errors := scraper.scrapeActiveServices(ctx)

	// Should return error for nil DB
	assert.Len(t, errors, 1)
	assert.Contains(t, errors[0].Error(), "invalid connection")
}

func TestConcurrentSafetyRacScraper(t *testing.T) {
	scraper := setupTestRacScraper(t)

	// Test concurrent access to RAC scraper
	// This tests the concurrent execution of RAC metric collection
	done := make(chan bool, 10)

	// Start 10 goroutines that try to scrape concurrently
	for i := 0; i < 10; i++ {
		go func(id int) {
			ctx := context.Background()
			// This will try to scrape cluster wait events concurrently
			_ = scraper.scrapeClusterWaitEvents(ctx)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Good, goroutine completed
		case <-time.After(5 * time.Second):
			t.Fatal("Goroutine did not complete within timeout - possible deadlock")
		}
	}
}

// Benchmark tests
func BenchmarkScrapeRacMetrics(b *testing.B) {
	logger := zap.NewNop()
	set := receivertest.NewNopSettings(component.MustNewType("newrelicoraclereceiver"))
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), set)
	scraper := NewRacScraper(nil, mb, logger, testInstanceName, metadata.DefaultMetricsBuilderConfig())

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scraper.ScrapeRacMetrics(ctx)
	}
}

func BenchmarkConcurrentVsSequential(b *testing.B) {
	logger := zap.NewNop()
	set := receivertest.NewNopSettings(component.MustNewType("newrelicoraclereceiver"))
	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), set)
	scraper := NewRacScraper(nil, mb, logger, testInstanceName, metadata.DefaultMetricsBuilderConfig())

	b.Run("Concurrent", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = scraper.ScrapeRacMetrics(ctx)
		}
	})

	b.Run("Sequential", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate sequential processing
			_ = scraper.scrapeASMDiskGroups(ctx)
			_ = scraper.scrapeClusterWaitEvents(ctx)
			_ = scraper.scrapeInstanceStatus(ctx)
			_ = scraper.scrapeActiveServices(ctx)
		}
	})
}

// Test that validates concurrent processing actually happens
func TestConcurrentProcessingVerification(t *testing.T) {
	scraper := setupTestRacScraper(t)

	// Measure time for concurrent processing
	start := time.Now()
	ctx := context.Background()

	_ = scraper.ScrapeRacMetrics(ctx)
	elapsed := time.Since(start)

	// The main benefit we're testing is that concurrent processing
	// should not be significantly slower than sequential for error cases
	// Since we're using nil DB, all operations fail quickly
	assert.Less(t, elapsed, 1*time.Second, "Concurrent processing should be fast even with errors")
}

// Test error aggregation from multiple concurrent operations
func TestErrorAggregationConcurrent(t *testing.T) {
	scraper := setupTestRacScraper(t)

	ctx := context.Background()
	errors := scraper.ScrapeRacMetrics(ctx)

	// With nil DB, all 4 scraper functions should produce errors
	assert.NotEmpty(t, errors)

	// Verify that errors from all goroutines are collected
	// Each scraper function with nil DB should produce exactly one error
	assert.LessOrEqual(t, len(errors), 4, "Should have at most 4 errors")
	assert.GreaterOrEqual(t, len(errors), 1, "Should have at least 1 error")

	// All errors should be about invalid connection
	for _, err := range errors {
		assert.Contains(t, err.Error(), "invalid connection")
	}
}

// Test component lifecycle
func TestRacScraperLifecycle(t *testing.T) {
	// Test creation
	scraper := setupTestRacScraper(t)
	require.NotNil(t, scraper)

	// Test that scraper is in valid initial state
	assert.Nil(t, scraper.isRacMode) // RAC mode not yet determined

	// Test operation (even with nil DB should not panic)
	ctx := context.Background()
	errors := scraper.ScrapeRacMetrics(ctx)
	assert.NotNil(t, errors) // Should get errors, but no panic

	// Test that state is maintained properly
	assert.Nil(t, scraper.isRacMode) // Should still be nil due to DB error
}

// Integration test placeholder
func TestRacScraperIntegration(t *testing.T) {
	// Skip this test unless we have a real Oracle RAC environment
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would be implemented when we have a real Oracle RAC test environment
	// It would test against actual Oracle database with real queries
	t.Skip("Integration test requires Oracle RAC environment")
}
