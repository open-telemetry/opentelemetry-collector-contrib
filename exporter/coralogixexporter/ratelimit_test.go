// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateError_ErrorCountReset(t *testing.T) {
	re := &rateError{
		threshold: 5,
		enabled:   true,
	}
	assert.Equal(t, int32(0), re.errorCount.Load())

	for i := 0; i < 5; i++ {
		re.enableRateLimit(nil)
	}
	assert.Equal(t, int32(5), re.errorCount.Load())

	re.rateLimited.Store(false)
	re.isRateLimited()

	assert.Equal(t, int32(0), re.errorCount.Load())
}

func TestRateError_ErrorCountNotResetWhenRateLimited(t *testing.T) {
	re := &rateError{
		threshold: 1,
		enabled:   true,
	}

	for i := 0; i < re.threshold; i++ {
		re.enableRateLimit(nil)
	}

	initialCount := re.errorCount.Load()
	re.isRateLimited()

	assert.Equal(t, initialCount, re.errorCount.Load())
}
