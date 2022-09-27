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

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func getNewAndPolicy(logger *zap.Logger, config *AndCfg) (sampling.PolicyEvaluator, error) {
	var subPolicyEvaluators []sampling.PolicyEvaluator
	for i := range config.SubPolicyCfg {
		policyCfg := &config.SubPolicyCfg[i]
		policy, err := getAndSubPolicyEvaluator(logger, policyCfg)
		if err != nil {
			return nil, err
		}
		subPolicyEvaluators = append(subPolicyEvaluators, policy)
	}
	return sampling.NewAnd(logger, subPolicyEvaluators), nil
}

// Return instance of and sub-policy
func getAndSubPolicyEvaluator(logger *zap.Logger, cfg *AndSubPolicyCfg) (sampling.PolicyEvaluator, error) {
	return getSharedPolicyEvaluator(logger, &cfg.sharedPolicyCfg)
}
