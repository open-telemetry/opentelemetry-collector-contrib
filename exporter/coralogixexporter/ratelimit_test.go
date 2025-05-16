// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func TestRateError_ErrorCountReset(t *testing.T) {
	re := &rateError{}

	for i := 0; i < 5; i++ {
		re.enableRateLimit(nil)
	}

	if re.errorCount.Load() == 0 {
		t.Error("Expected error count to be non-zero after enabling rate limit")
	}

	re.isRateLimited()

	if re.errorCount.Load() != 0 {
		t.Errorf("Expected error count to be reset to 0, got %d", re.errorCount.Load())
	}
}

func TestRateError_ErrorCountNotResetWhenRateLimited(t *testing.T) {
	re := &rateError{}

	for i := 0; i < int(rateLimitThreshold); i++ {
		re.enableRateLimit(nil)
	}

	initialCount := re.errorCount.Load()

	prevEnableRateLimiterFeatureGateValue := enableRateLimiterFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(enableRateLimiterFeatureGateID, prevEnableRateLimiterFeatureGateValue))
	}()

	re.isRateLimited()

	if re.errorCount.Load() != initialCount {
		t.Errorf("Expected error count to remain unchanged, got %d, want %d", re.errorCount.Load(), initialCount)
	}
}
