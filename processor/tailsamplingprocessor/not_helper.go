// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func getNewNotPolicy(settings component.TelemetrySettings, config *NotCfg, policyExtensions map[string]samplingpolicy.Extension) (samplingpolicy.Evaluator, error) {
	subPolicyEvaluator, err := getNotSubPolicyEvaluator(settings, &config.SubPolicy, policyExtensions)
	if err != nil {
		return nil, err
	}
	return sampling.NewNot(settings.Logger, subPolicyEvaluator), nil
}

// Return instance of not sub-policy
func getNotSubPolicyEvaluator(settings component.TelemetrySettings, cfg *NotSubPolicyCfg, policyExtensions map[string]samplingpolicy.Extension) (samplingpolicy.Evaluator, error) {
	return getSharedPolicyEvaluator(settings, &cfg.sharedPolicyCfg, policyExtensions)
}
