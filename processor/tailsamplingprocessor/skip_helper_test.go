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

func TestSkipHelper(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		actual, err := getNewSkipPolicy(componenttest.NewNopTelemetrySettings(), &SkipCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-skip-policy-1",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 100},
					},
				},
			},
		})
		require.NoError(t, err)

		expected := sampling.NewSkip(zap.NewNop(), []sampling.PolicyEvaluator{
			sampling.NewLatency(componenttest.NewNopTelemetrySettings(), 100, 0),
		})
		assert.Equal(t, expected, actual)
	})

	t.Run("multiple sub-policies", func(t *testing.T) {
		actual, err := getNewSkipPolicy(componenttest.NewNopTelemetrySettings(), &SkipCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-skip-policy-latency",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 200},
					},
				},
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-skip-policy-string",
						Type: StringAttribute,
						StringAttributeCfg: StringAttributeCfg{
							Key:    "service.name",
							Values: []string{"test-service"},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.NotNil(t, actual)
	})

	t.Run("unsupported sampling policy type", func(t *testing.T) {
		_, err := getNewSkipPolicy(componenttest.NewNopTelemetrySettings(), &SkipCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-skip-policy-2",
						Type: Skip, // nested skip is not allowed
					},
				},
			},
		})
		require.EqualError(t, err, "unknown sampling policy type skip")
	})

	t.Run("unsupported drop policy type", func(t *testing.T) {
		_, err := getNewSkipPolicy(componenttest.NewNopTelemetrySettings(), &SkipCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-skip-policy-3",
						Type: Drop, // nested drop is not allowed
					},
				},
			},
		})
		require.EqualError(t, err, "unknown sampling policy type drop")
	})

	t.Run("empty sub-policy config", func(t *testing.T) {
		actual, err := getNewSkipPolicy(componenttest.NewNopTelemetrySettings(), &SkipCfg{
			SubPolicyCfg: []AndSubPolicyCfg{},
		})
		require.NoError(t, err)

		expected := sampling.NewSkip(zap.NewNop(), []sampling.PolicyEvaluator{})
		assert.Equal(t, expected, actual)
	})

	t.Run("valid probabilistic policy", func(t *testing.T) {
		actual, err := getNewSkipPolicy(componenttest.NewNopTelemetrySettings(), &SkipCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-skip-policy-probabilistic",
						Type: Probabilistic,
						ProbabilisticCfg: ProbabilisticCfg{
							SamplingPercentage: 50.0,
						},
					},
				},
			},
		})
		require.NoError(t, err)

		expected := sampling.NewSkip(zap.NewNop(), []sampling.PolicyEvaluator{
			sampling.NewProbabilisticSampler(componenttest.NewNopTelemetrySettings(), "", 50.0),
		})
		assert.Equal(t, expected, actual)
	})
}

func TestGetSkipSubPolicyEvaluator(t *testing.T) {
	t.Run("valid latency policy", func(t *testing.T) {
		cfg := &AndSubPolicyCfg{
			sharedPolicyCfg: sharedPolicyCfg{
				Name:       "test-latency",
				Type:       Latency,
				LatencyCfg: LatencyCfg{ThresholdMs: 300},
			},
		}

		actual, err := getSkipSubPolicyEvaluator(componenttest.NewNopTelemetrySettings(), cfg)
		require.NoError(t, err)

		expected := sampling.NewLatency(componenttest.NewNopTelemetrySettings(), 300, 0)
		assert.Equal(t, expected, actual)
	})

	t.Run("valid status code policy", func(t *testing.T) {
		cfg := &AndSubPolicyCfg{
			sharedPolicyCfg: sharedPolicyCfg{
				Name:          "test-status",
				Type:          StatusCode,
				StatusCodeCfg: StatusCodeCfg{StatusCodes: []string{"ERROR", "OK"}},
			},
		}

		actual, err := getSkipSubPolicyEvaluator(componenttest.NewNopTelemetrySettings(), cfg)
		require.NoError(t, err)

		expected, _ := sampling.NewStatusCodeFilter(componenttest.NewNopTelemetrySettings(), []string{"ERROR", "OK"})
		assert.Equal(t, expected, actual)
	})

	t.Run("invalid policy type", func(t *testing.T) {
		cfg := &AndSubPolicyCfg{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "test-invalid",
				Type: PolicyType("invalid_policy"), // invalid policy type
			},
		}

		_, err := getSkipSubPolicyEvaluator(componenttest.NewNopTelemetrySettings(), cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown sampling policy type")
	})
}
