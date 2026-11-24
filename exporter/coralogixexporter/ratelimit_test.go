// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateError_ErrorCountNotResetWhenRateLimited(t *testing.T) {
	re := &rateError{
		threshold: 1,
		enabled:   true,
		duration:  time.Minute,
	}

	for i := 0; i < re.threshold; i++ {
		re.enableRateLimit(dropReasonAuth, "Unauthenticated")
	}

	initialCount := re.errorCount.Load()
	re.isRateLimited()

	assert.Equal(t, initialCount, re.errorCount.Load())
}

func TestRateError_EnableRateLimitOpensOnce(t *testing.T) {
	re := &rateError{
		threshold: 2,
		enabled:   true,
		duration:  time.Minute,
	}

	assert.False(t, re.enableRateLimit(dropReasonAuth, "Unauthenticated"))
	assert.False(t, re.isRateLimited())

	assert.True(t, re.enableRateLimit(dropReasonAuth, "Unauthenticated"))
	require.True(t, re.isRateLimited())
	assert.Contains(t, re.GetError().Error(), "authentication failed")
	assert.NotContains(t, re.GetError().Error(), "rate limit exceeded")
	assert.Equal(t, dropReasonAuth, re.getReason())

	assert.False(t, re.enableRateLimit(dropReasonAuth, "Unauthenticated"))
}

func TestRateError_QuotaErrorText(t *testing.T) {
	re := &rateError{
		threshold: 1,
		enabled:   true,
		duration:  time.Minute,
	}

	assert.True(t, re.enableRateLimit(dropReasonQuota, "ResourceExhausted"))
	assert.Contains(t, re.GetError().Error(), "quota exceeded")
	assert.NotContains(t, re.GetError().Error(), "rate limit exceeded")
}

func TestRateError_DisableRateLimitReportsClose(t *testing.T) {
	re := &rateError{
		threshold: 1,
		enabled:   true,
		duration:  time.Minute,
	}

	assert.True(t, re.enableRateLimit(dropReasonAuth, "Unauthenticated"))
	assert.True(t, re.disableRateLimit())
	assert.False(t, re.isRateLimited())
	assert.False(t, re.disableRateLimit())
}
