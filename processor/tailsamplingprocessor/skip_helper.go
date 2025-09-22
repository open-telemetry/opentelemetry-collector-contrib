// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func getNewSkipPolicy(settings component.TelemetrySettings, config *SkipCfg) (samplingpolicy.Evaluator, error) {
	subPolicyEvaluators := make([]samplingpolicy.Evaluator, len(config.SubPolicyCfg))
	for i := range config.SubPolicyCfg {
		policyCfg := &config.SubPolicyCfg[i]
		policy, err := getSkipSubPolicyEvaluator(settings, policyCfg)
		if err != nil {
			return nil, err
		}
		subPolicyEvaluators[i] = policy
	}
	return sampling.NewSkip(subPolicyEvaluators), nil
}

// Return instance of and sub-policy
func getSkipSubPolicyEvaluator(settings component.TelemetrySettings, cfg *AndSubPolicyCfg) (samplingpolicy.Evaluator, error) {
	return getSharedPolicyEvaluator(settings, &cfg.sharedPolicyCfg)
}
