// Copyright  The OpenTelemetry Authors
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

func TestAndHelper(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		actual, err := getNewAndPolicy(zap.NewNop(), &AndCfg{
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
			sampling.NewLatency(zap.NewNop(), 100),
		})
		assert.Equal(t, expected, actual)
	})

	t.Run("unsupported sampling policy type", func(t *testing.T) {
		_, err := getNewAndPolicy(zap.NewNop(), &AndCfg{
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
