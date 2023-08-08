// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func getNewCompositePolicy(settings component.TelemetrySettings, config *CompositeCfg) (sampling.PolicyEvaluator, error) {
	subPolicyEvalParams := make([]sampling.SubPolicyEvalParams, len(config.SubPolicyCfg))
	rateAllocationsMap := getRateAllocationMap(config)
	for i := range config.SubPolicyCfg {
		policyCfg := &config.SubPolicyCfg[i]
		policy, err := getCompositeSubPolicyEvaluator(settings, policyCfg)
		if err != nil {
			return nil, err
		}

		evalParams := sampling.SubPolicyEvalParams{
			Evaluator:         policy,
			MaxSpansPerSecond: int64(rateAllocationsMap[policyCfg.Name]),
		}
		subPolicyEvalParams[i] = evalParams
	}
	return sampling.NewComposite(settings.Logger, config.MaxTotalSpansPerSecond, subPolicyEvalParams, sampling.MonotonicClock{}), nil
}

// Apply rate allocations to the sub-policies
func getRateAllocationMap(config *CompositeCfg) map[string]float64 {
	rateAllocationsMap := make(map[string]float64)
	maxTotalSPS := float64(config.MaxTotalSpansPerSecond)
	// Default SPS determined by equally diving number of sub policies
	defaultSPS := maxTotalSPS / float64(len(config.SubPolicyCfg))
	for _, rAlloc := range config.RateAllocation {
		if rAlloc.Percent > 0 {
			rateAllocationsMap[rAlloc.Policy] = (float64(rAlloc.Percent) / 100) * maxTotalSPS
		} else {
			rateAllocationsMap[rAlloc.Policy] = defaultSPS
		}
	}
	return rateAllocationsMap
}

// Return instance of composite sub-policy
func getCompositeSubPolicyEvaluator(settings component.TelemetrySettings, cfg *CompositeSubPolicyCfg) (sampling.PolicyEvaluator, error) {
	switch cfg.Type {
	case And:
		return getNewAndPolicy(settings, &cfg.AndCfg)
	default:
		return getSharedPolicyEvaluator(settings, &cfg.sharedPolicyCfg)
	}
}
