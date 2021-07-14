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

package metricsgenerationprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type metricsGenerationProcessor struct {
	rules  []internalRule
	logger *zap.Logger
}

type internalRule struct {
	name      string
	unit      string
	ruleType  string
	metric1   string
	metric2   string
	operation string
	scaleBy   float64
}

func newMetricsGenerationProcessor(rules []internalRule, logger *zap.Logger) *metricsGenerationProcessor {
	return &metricsGenerationProcessor{
		rules:  rules,
		logger: logger,
	}
}

// Start is invoked during service startup.
func (mgp *metricsGenerationProcessor) Start(context.Context, component.Host) error {
	return nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (mgp *metricsGenerationProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	resourceMetricsSlice := md.ResourceMetrics()

	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		nameToMetricMap := getNameToMetricMap(rm)

		for _, rule := range mgp.rules {
			operand2 := float64(0)
			_, ok := nameToMetricMap[rule.metric1]
			if !ok {
				mgp.logger.Debug("Missing first metric", zap.String("metric_name", rule.metric1))
				continue
			}

			if rule.ruleType == string(calculate) {
				metric2, ok := nameToMetricMap[rule.metric2]
				if !ok {
					mgp.logger.Debug("Missing second metric", zap.String("metric_name", rule.metric2))
					continue
				}
				operand2 = getMetricValue(metric2)
				if operand2 <= 0 {
					continue
				}

			} else if rule.ruleType == string(scale) {
				operand2 = rule.scaleBy
			}
			generateMetrics(rm, operand2, rule, mgp.logger)
		}
	}
	return md, nil
}

// Shutdown is invoked during service shutdown.
func (mgp *metricsGenerationProcessor) Shutdown(context.Context) error {
	return nil
}
