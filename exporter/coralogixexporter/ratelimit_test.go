// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package coralogixexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRateError_ErrorCountNotResetWhenRateLimited(t *testing.T) {
	re := &rateError{
		threshold: 1,
		enabled:   true,
	}

	for i := 0; i < re.threshold; i++ {
		re.enableRateLimit()
	}

	initialCount := re.errorCount.Load()
	re.isRateLimited()

	assert.Equal(t, initialCount, re.errorCount.Load())
}
