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
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func getNewAndPolicy(settings component.TelemetrySettings, config *AndCfg) (sampling.PolicyEvaluator, error) {
	var subPolicyEvaluators []sampling.PolicyEvaluator
	for i := range config.SubPolicyCfg {
		policyCfg := &config.SubPolicyCfg[i]
		policy, err := getAndSubPolicyEvaluator(settings, policyCfg)
		if err != nil {
			return nil, err
		}
		subPolicyEvaluators = append(subPolicyEvaluators, policy)
	}
	return sampling.NewAnd(settings.Logger, subPolicyEvaluators), nil
}

// Return instance of and sub-policy
func getAndSubPolicyEvaluator(settings component.TelemetrySettings, cfg *AndSubPolicyCfg) (sampling.PolicyEvaluator, error) {
	return getSharedPolicyEvaluator(settings, &cfg.sharedPolicyCfg)
}
