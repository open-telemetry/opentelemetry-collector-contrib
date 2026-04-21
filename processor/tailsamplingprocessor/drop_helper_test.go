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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestDropHelper(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		actual, err := getNewDropPolicy(componenttest.NewNopTelemetrySettings(), &DropCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name:       "test-and-policy-1",
						Type:       Latency,
						LatencyCfg: LatencyCfg{ThresholdMs: 100},
					},
				},
			},
		}, nil)
		require.NoError(t, err)

		expected := sampling.NewDrop(zap.NewNop(), []samplingpolicy.Evaluator{
			sampling.NewLatency(componenttest.NewNopTelemetrySettings(), 100, 0),
		})
		assert.Equal(t, expected, actual)
	})

	t.Run("unsupported sampling policy type", func(t *testing.T) {
		_, err := getNewDropPolicy(componenttest.NewNopTelemetrySettings(), &DropCfg{
			SubPolicyCfg: []AndSubPolicyCfg{
				{
					sharedPolicyCfg: sharedPolicyCfg{
						Name: "test-and-policy-2",
						Type: Drop, // nested drop is not allowed
					},
				},
			},
		}, nil)
		require.EqualError(t, err, "unknown sampling policy type drop")
	})
}
