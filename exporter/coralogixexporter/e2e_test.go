// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package coralogixexporter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func Test_E2E_RateLimit_HappyPath(t *testing.T) {
	privateKey := os.Getenv("CORALOGIX_PRIVATE_KEY")
	if privateKey == "" {
		t.Skip("Skipping E2E test: CORALOGIX_PRIVATE_KEY not set")
	}

	cfg := &Config{
		Domain:     "eu2.coralogix.com",
		PrivateKey: configopaque.String(privateKey),
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 5,
			Duration:  30 * time.Second,
		},
		AppName:   "e2e-test-app",
		SubSystem: "e2e-test-subsystem",
	}

	t.Log("Creating exporter")
	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	t.Log("Starting exporter")
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		t.Log("Shutting down exporter")
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	start := time.Now()

	t.Log("Sending data for 5 mins")
	count := 0
	traces := generateTestTraces()
	for time.Since(start) < 5*time.Minute {
		err = exp.pushTraces(context.Background(), traces)
		require.NoError(t, err)
		if count%10 == 0 {
			t.Logf("Sending traces %d", count)
		}
		count++
	}
}

func Test_E2E_InvalidKeyRateLimit(t *testing.T) {
	privateKey := os.Getenv("CORALOGIX_PRIVATE_KEY_LOW_QUOTA")
	if privateKey == "" {
		t.Skip("Skipping E2E test: CORALOGIX_PRIVATE_KEY_LOW_QUOTA not set")
	}

	cfg := &Config{
		Domain:     "eu2.coralogix.com",
		PrivateKey: configopaque.String("invalid-key"),
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 5,
			Duration:  30 * time.Second,
		},
		AppName:   "e2e-test-app",
		SubSystem: "e2e-test-subsystem",
	}

	t.Log("Creating exporter")
	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	t.Log("Starting exporter")
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		t.Log("Shutting down exporter")
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Log("Phase 1: Sending data until rate limiter is activated")
	rateLimited := false
	errorCount := 0
	rateLimitedCount := 0

	for !rateLimited {
		traces := generateTestTraces()
		err = exp.pushTraces(context.Background(), traces)
		if err != nil {
			errorCount++
			if exp.rateError.isRateLimited() {
				rateLimitedCount++
				rateLimited = true
				t.Logf("Rate limiter activated! Error count: %d, Rate limited count: %d", errorCount, rateLimitedCount)
			} else {
				t.Logf("Received error (not rate limited yet). Error count: %d", errorCount)
			}
		}
	}

	require.True(t, rateLimited, "Rate limiter should have been activated")

	t.Log("Phase 2: trying to send data during rate limit period (but will be rate limited)")
	rateLimitStart := time.Now()
	rateLimitEnabled := false

	count := 0
	for time.Since(rateLimitStart) < cfg.RateLimiter.Duration {
		traces := generateTestTraces()
		err = exp.pushTraces(context.Background(), traces)
		if err != nil && exp.rateError.isRateLimited() {
			if count%5000 == 0 { // Do not spam the logs
				t.Logf("Tried to send data but limited for %s", cfg.RateLimiter.Duration-time.Since(*exp.rateError.timestamp.Load()))
			}
			rateLimitEnabled = true
		} else if exp.rateError.isRateLimited() {
			t.Fatalf("Should have returned error")
		}
		count++
	}

	require.True(t, rateLimitEnabled, "Should have received rate limit errors")

	t.Log("Phase 3: Verifying we can try to send data again")
	assert.Eventually(t, func() bool {
		return exp.canSend()
	}, 2*time.Minute, 100*time.Millisecond, "Should be able to send data after rate limiter duration")
}

func Test_E2E_LowQuotaRateLimit(t *testing.T) {
	privateKey := os.Getenv("CORALOGIX_PRIVATE_KEY_LOW_QUOTA")
	if privateKey == "" {
		t.Skip("Skipping E2E test: CORALOGIX_PRIVATE_KEY_LOW_QUOTA not set")
	}

	cfg := &Config{
		Domain:     "eu2.coralogix.com",
		PrivateKey: configopaque.String(privateKey),
		RateLimiter: RateLimiterConfig{
			Enabled:   true,
			Threshold: 5,
			Duration:  30 * time.Second,
		},
		AppName:   "e2e-test-app",
		SubSystem: "e2e-test-subsystem",
	}

	t.Log("Creating exporter")
	exp, err := newTracesExporter(cfg, exportertest.NewNopSettings(exportertest.NopType))
	require.NoError(t, err)

	t.Log("Starting exporter")
	err = exp.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		t.Log("Shutting down exporter")
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	}()

	t.Log("Phase 1: Sending data until rate limiter is activated")
	rateLimited := false
	errorCount := 0
	rateLimitedCount := 0

	for !rateLimited {
		traces := generateTestTraces()
		err = exp.pushTraces(context.Background(), traces)
		if err != nil {
			errorCount++
			if exp.rateError.isRateLimited() {
				rateLimitedCount++
				rateLimited = true
				t.Logf("Rate limiter activated! Error count: %d, Rate limited count: %d", errorCount, rateLimitedCount)
			} else {
				t.Logf("Received error (not rate limited yet). Error count: %d", errorCount)
			}
		}
	}

	require.True(t, rateLimited, "Rate limiter should have been activated")

	t.Log("Phase 2: trying to send data during rate limit period (but will be rate limited)")
	rateLimitEnabled := false

	count := 0
	for time.Since(*exp.rateError.timestamp.Load()) < cfg.RateLimiter.Duration {
		traces := generateTestTraces()
		err = exp.pushTraces(context.Background(), traces)
		if err != nil && exp.rateError.isRateLimited() {
			if count%5000 == 0 { // Do not spam the logs
				t.Logf("Tried to send data but limited for %s", cfg.RateLimiter.Duration-time.Since(*exp.rateError.timestamp.Load()))
			}
			rateLimitEnabled = true
		} else if exp.rateError.isRateLimited() {
			t.Fatalf("Should have returned error")
		}
		count++
	}

	require.True(t, rateLimitEnabled, "Should have received rate limit errors")

	t.Log("Phase 3: Verifying we can try to send data again")
	assert.Eventually(t, func() bool {
		return exp.canSend()
	}, 2*time.Minute, 100*time.Millisecond, "Should be able to send data after rate limiter duration")
}

func generateTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()

	for i := 0; i < 5000; i++ {
		rs := resourceSpans.AppendEmpty()
		resource := rs.Resource()
		resource.Attributes().PutStr("service.name", "e2e-test-service")
		resource.Attributes().PutStr("environment", "test")
		scopeSpans := rs.ScopeSpans().AppendEmpty()
		span := scopeSpans.Spans().AppendEmpty()
		span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
		span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
		span.SetName("test-span")
		span.SetKind(ptrace.SpanKindServer)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
	}

	return traces
}
