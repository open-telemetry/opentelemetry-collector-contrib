// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var matchAttributes = featuregate.GlobalRegistry().MustRegister(
	"metricsgeneration.MatchAttributes",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the metric calculations will only be done between data points whose attributes match."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/35425"),
	featuregate.WithRegisterFromVersion("v0.112.0"),
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
func (mgp *metricsGenerationProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	resourceMetricsSlice := md.ResourceMetrics()

	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		nameToMetricMap := getNameToMetricMap(rm)

		for _, rule := range mgp.rules {
			_, ok := nameToMetricMap[rule.metric1]
			if !ok {
				mgp.logger.Debug("Missing first metric", zap.String("metric_name", rule.metric1))
				continue
			}

			switch rule.ruleType {
			case string(calculate):
				// Operation type is validated during config validation, but this adds extra validation as a safety net
				ot := OperationType(rule.operation)
				if !ot.isValid() {
					mgp.logger.Debug(fmt.Sprintf("Invalid operation type '%s' specified for rule: %s. This rule is skipped.", rule.operation, rule.name))
					continue
				}

				metric2, ok := nameToMetricMap[rule.metric2]
				if !ok {
					mgp.logger.Debug("Missing second metric", zap.String("metric_name", rule.metric2))
					continue
				}

				if matchAttributes.IsEnabled() {
					generateCalculatedMetrics(rm, metric2, rule, mgp.logger)
				} else {
					// When matching metric attributes isn't required the value of the first data point of metric2 is
					// used for all calculations. The resulting logic is the same as generating a new metric from
					// a scalar.
					generateScalarMetrics(rm, getMetricValue(metric2), rule, mgp.logger)
				}
			case string(scale):
				generateScalarMetrics(rm, rule.scaleBy, rule, mgp.logger)
			default:
				mgp.logger.Error(fmt.Sprintf("Invalid rule type configured: '%s'. This rule is skipped.", rule.ruleType))
				continue
			}
		}
	}
	return md, nil
}

// Shutdown is invoked during service shutdown.
func (mgp *metricsGenerationProcessor) Shutdown(context.Context) error {
	return nil
}
