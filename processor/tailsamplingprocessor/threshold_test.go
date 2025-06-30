// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

// TestThresholdValues verifies the actual threshold values using pkg/sampling
func TestThresholdValues(t *testing.T) {
	testCases := []string{"0", "8", "f", "ff", "fff", "ffff"}

	for _, tValue := range testCases {
		threshold, err := sampling.TValueToThreshold(tValue)
		if err != nil {
			t.Errorf("Failed to parse threshold %s: %v", tValue, err)
			continue
		}

		adjustedCount := threshold.AdjustedCount()
		probability := threshold.Probability()

		t.Logf("th:%s -> AdjustedCount: %.6f, Probability: %.10f",
			tValue, adjustedCount, probability)
	}
}
