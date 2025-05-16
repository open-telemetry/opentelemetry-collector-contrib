// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixexporter

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
)

func TestSignalExporter_CanSend_AfterRateLimitTimeout(t *testing.T) {
	prevEnableRateLimiterFeatureGateValue := enableRateLimiterFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, true))
	prevRateLimitThreshold := rateLimitThreshold
	rateLimitThreshold = 1
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, prevEnableRateLimiterFeatureGateValue))
		rateLimitThreshold = prevRateLimitThreshold
	}()

	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	rateLimitErr := errors.New("rate limit exceeded")
	exp.EnableRateLimit(rateLimitErr)
	exp.EnableRateLimit(rateLimitErr)

	assert.False(t, exp.canSend())

	// Mock the time to be after the rate limit timeout (1 minute)
	exp.rateError.state.Store(&rateErrorState{
		timestamp: time.Now().Add(-2 * time.Minute), // Set timestamp to 2 minutes ago
		err:       rateLimitErr,
	})

	assert.True(t, exp.canSend())
	assert.False(t, exp.rateError.isRateLimited())
}

func TestSignalExporter_CanSend_FeatureGateDisabled(t *testing.T) {
	prevEnableRateLimiterFeatureGateValue := enableRateLimiterFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, false))
	prevRateLimitThreshold := rateLimitThreshold
	rateLimitThreshold = 1
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, prevEnableRateLimiterFeatureGateValue))
		rateLimitThreshold = prevRateLimitThreshold
	}()

	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	rateLimitErr := errors.New("rate limit exceeded")
	exp.EnableRateLimit(rateLimitErr)
	exp.EnableRateLimit(rateLimitErr)

	assert.True(t, exp.canSend())
}

func TestSignalExporter_CanSend_BeforeRateLimitTimeout(t *testing.T) {
	prevEnableRateLimiterFeatureGateValue := enableRateLimiterFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, true))
	prevRateLimitThreshold := rateLimitThreshold
	rateLimitThreshold = 1
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, prevEnableRateLimiterFeatureGateValue))
		rateLimitThreshold = prevRateLimitThreshold
	}()

	cfg := &Config{
		Domain:     "test.domain.com",
		PrivateKey: "test-key",
	}

	exp, err := newSignalExporter(cfg, exportertest.NewNopSettings(exportertest.NopType), "", nil)
	require.NoError(t, err)

	rateLimitErr := errors.New("rate limit exceeded")
	exp.EnableRateLimit(rateLimitErr)
	exp.EnableRateLimit(rateLimitErr)

	// Mock the time to be before the rate limit timeout (30 seconds ago)
	exp.rateError.state.Store(&rateErrorState{
		timestamp: time.Now().Add(-30 * time.Second),
		err:       rateLimitErr,
	})

	// Should not be able to send because we're still within the timeout period
	assert.False(t, exp.canSend())
	assert.True(t, exp.rateError.isRateLimited())
}
