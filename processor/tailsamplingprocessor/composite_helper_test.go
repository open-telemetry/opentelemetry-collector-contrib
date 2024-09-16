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

func TestCompositeHelper(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		actual, err := getNewCompositePolicy(componenttest.NewNopTelemetrySettings(), &CompositeCfg{
			MaxTotalSpansPerSecond: 1000,
			PolicyOrder:            []string{"test-composite-policy-1"},
			SubPolicyCfg: []CompositeSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-composite-policy-1",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 100},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-composite-policy-2",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 200},
					},
				},
			},
			RateAllocation: []RateAllocationCfg{
				{
					Policy:  "test-composite-policy-1",
					Percent: 25,
				},
				{
					Policy:  "test-composite-policy-2",
					Percent: 0, // will be populated with default
				},
			},
		})
		require.NoError(t, err)

		expected := sampling.NewComposite(zap.NewNop(), 1000, []sampling.SubPolicyEvalParams{
			{
				Evaluator:         sampling.NewLatency(componenttest.NewNopTelemetrySettings(), 100, 0),
				MaxSpansPerSecond: 250,
			},
			{
				Evaluator:         sampling.NewLatency(componenttest.NewNopTelemetrySettings(), 200, 0),
				MaxSpansPerSecond: 500,
			},
		}, sampling.MonotonicClock{})
		assert.Equal(t, expected, actual)
	})

	t.Run("unsupported sampling policy type", func(t *testing.T) {
		_, err := getNewCompositePolicy(componenttest.NewNopTelemetrySettings(), &CompositeCfg{
			SubPolicyCfg: []CompositeSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-composite-policy-1",
						Type: Composite, // nested composite is not allowed
					},
				},
			},
		})
		require.EqualError(t, err, "unknown sampling policy type composite")
	})
}
