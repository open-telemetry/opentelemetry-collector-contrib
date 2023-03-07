// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func TestCompositeHelper(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		actual, err := getNewCompositePolicy(zap.NewNop(), &CompositeCfg{
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
				Evaluator:         sampling.NewLatency(zap.NewNop(), 100),
				MaxSpansPerSecond: 250,
			},
			{
				Evaluator:         sampling.NewLatency(zap.NewNop(), 200),
				MaxSpansPerSecond: 500,
			},
		}, sampling.MonotonicClock{})
		assert.Equal(t, expected, actual)
	})

	t.Run("unsupported sampling policy type", func(t *testing.T) {
		_, err := getNewCompositePolicy(zap.NewNop(), &CompositeCfg{
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
