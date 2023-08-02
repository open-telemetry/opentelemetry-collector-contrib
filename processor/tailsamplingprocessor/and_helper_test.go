// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func TestAndHelper(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		actual, err := getNewAndPolicy(componenttest.NewNopTelemetrySettings(), &AndCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-and-policy-1",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 100},
					},
				},
			},
		})
		require.NoError(t, err)

		expected := sampling.NewAnd(zap.NewNop(), []sampling.PolicyEvaluator{
			sampling.NewLatency(componenttest.NewNopTelemetrySettings(), 100),
		})
		assert.Equal(t, expected, actual)
	})

	t.Run("unsupported sampling policy type", func(t *testing.T) {
		_, err := getNewAndPolicy(componenttest.NewNopTelemetrySettings(), &AndCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-and-policy-2",
						Type: And, // nested and is not allowed
					},
				},
			},
		})
		require.EqualError(t, err, "unknown sampling policy type and")
	})
}
