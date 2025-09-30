// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoraclereceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestScraperShutdown_NilDB(t *testing.T) {
	// Create scraper with nil database
	scraper := &newRelicOracleScraper{
		db:     nil,
		logger: zap.NewNop(),
	}

	// Test shutdown with nil database
	err := scraper.shutdown(context.Background())
	assert.NoError(t, err) // Should not error when db is nil
}

func TestConcurrentProcessingTimeout(t *testing.T) {
	// Test that context timeout is properly handled
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Let the context timeout
	time.Sleep(5 * time.Millisecond)

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
		assert.Contains(t, ctx.Err().Error(), "deadline exceeded")
	default:
		t.Fatal("Expected context to be cancelled")
	}
}

func TestScraperFuncSignature(t *testing.T) {
	// Test that ScraperFunc type is correctly defined
	var scraperFunc ScraperFunc

	// Create a test function that matches the signature
	testFunc := func(ctx context.Context) []error {
		return []error{assert.AnError}
	}

	// Should be able to assign to ScraperFunc
	scraperFunc = testFunc
	assert.NotNil(t, scraperFunc)

	// Should be able to call it
	errors := scraperFunc(context.Background())
	assert.Len(t, errors, 1)
	assert.Equal(t, assert.AnError, errors[0])
}

func TestErrorChannelHandling(t *testing.T) {
	// Test error channel capacity and handling
	const maxErrors = 100
	errChan := make(chan error, maxErrors)

	// Fill channel to capacity
	for i := 0; i < maxErrors; i++ {
		errChan <- assert.AnError
	}

	// Channel should be at capacity
	assert.Equal(t, maxErrors, len(errChan))

	// Try to add one more (should not block with proper handling)
	select {
	case errChan <- assert.AnError:
		t.Fatal("Expected channel to be full")
	default:
		// Expected behavior - channel is full
	}

	close(errChan)

	// Count errors received
	var errorCount int
	for range errChan {
		errorCount++
	}

	assert.Equal(t, maxErrors, errorCount)
}

func TestContextCancellation(t *testing.T) {
	// Test that context cancellation is properly detected
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		assert.Error(t, ctx.Err())
		assert.Equal(t, context.Canceled, ctx.Err())
	default:
		t.Fatal("Expected context to be cancelled")
	}
}

func TestScraperConstants(t *testing.T) {
	// Test that constants are properly defined
	assert.Equal(t, 30*time.Second, keepAlive)
}

func BenchmarkErrorChannelOperations(b *testing.B) {
	// Benchmark error channel operations for performance
	const maxErrors = 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errChan := make(chan error, maxErrors)

		// Simulate concurrent error sending
		for j := 0; j < 10; j++ {
			errChan <- assert.AnError
		}

		close(errChan)

		// Collect errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}
	}
}
