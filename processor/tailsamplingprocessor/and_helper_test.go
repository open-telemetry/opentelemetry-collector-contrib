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

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAndHelper(t *testing.T) {
	andCfg := &AndCfg{
		SubPolicyCfg: []AndSubPolicyCfg{
			{
				Name: "test-and-policy-1",
				Type: AlwaysSample,
			},
			{
				Name:                "test-and-policy-2",
				Type:                NumericAttribute,
				NumericAttributeCfg: NumericAttributeCfg{Key: "key1", MinValue: 50, MaxValue: 100},
			},
			{
				Name:               "test-and-policy-3",
				Type:               StringAttribute,
				StringAttributeCfg: StringAttributeCfg{Key: "key2", Values: []string{"value1", "value2"}},
			},
			{
				Name:            "test-and-policy-4",
				Type:            RateLimiting,
				RateLimitingCfg: RateLimitingCfg{SpansPerSecond: 10},
			},
			{
				Name:          "test-and-policy-5",
				Type:          StatusCode,
				StatusCodeCfg: StatusCodeCfg{StatusCodes: []string{"ERROR", "UNSET"}},
			},
			{
				Name:             "test-and-policy-6",
				Type:             Probabilistic,
				ProbabilisticCfg: ProbabilisticCfg{HashSalt: "salt", SamplingPercentage: 10},
			},
			{
				Name:          "test-and-policy-3",
				Type:          TraceState,
				TraceStateCfg: TraceStateCfg{Key: "key3", Values: []string{"value1", "value2"}},
			},
			{
				Name:         "test-and-policy-1",
				Type:         SpanCount,
				SpanCountCfg: SpanCountCfg{MinSpans: 2},
			},
		},
	}

	for i := range andCfg.SubPolicyCfg {
		policy, e := getAndSubPolicyEvaluator(zap.NewNop(), &andCfg.SubPolicyCfg[i])
		require.NotNil(t, policy)
		require.NoError(t, e)
	}
}
