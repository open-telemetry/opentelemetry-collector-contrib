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
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

func TestAndHelper(t *testing.T) {
	cfg := &Config{
		ProcessorSettings:       config.NewProcessorSettings(config.NewComponentID(typeStr)),
		DecisionWait:            10 * time.Second,
		NumTraces:               100,
		ExpectedNewTracesPerSec: 10,
		PolicyCfgs: []PolicyCfg{
			{
				Name: "and-policy-1",
				Type: And,
				AndCfg: AndCfg{
					SubPolicyCfg: []AndSubPolicyCfg{
						{
							Name:         "test-and-policy-1",
							Type:         SpanCount,
							SpanCountCfg: SpanCountCfg{MinSpans: 2},
						},
					},
				},
			},
		},
	}

	andCfg := cfg.PolicyCfgs[0].AndCfg
	andSubPolicyConfig := andCfg.SubPolicyCfg[0]
	result, e := getAndSubPolicyEvaluator(zap.NewNop(), &andSubPolicyConfig)
	require.NotNil(t, result)
	require.NoError(t, e)
}
