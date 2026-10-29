// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatorateprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type deltaToRateProcessor struct {
	ConfiguredMetrics map[string]bool
	logger            *zap.Logger
}

func newDeltaToRateProcessor(config *Config, logger *zap.Logger) *deltaToRateProcessor {
	inputMetricSet := make(map[string]bool, len(config.Metrics))
	for _, name := range config.Metrics {
		inputMetricSet[name] = true
	}

	return &deltaToRateProcessor{
		ConfiguredMetrics: inputMetricSet,
		logger:            logger,
	}
}

// Start is invoked during service startup.
func (*deltaToRateProcessor) Start(context.Context, component.Host) error {
	return nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (dtrp *deltaToRateProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	resourceMetricsSlice := md.ResourceMetrics()

	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		ilms := rm.ScopeMetrics()
		for i := 0; i < ilms.Len(); i++ {
			ilm := ilms.At(i)
			metricSlice := ilm.Metrics()
			for j := 0; j < metricSlice.Len(); j++ {
				metric := metricSlice.At(j)
				if _, ok := dtrp.ConfiguredMetrics[metric.Name()]; !ok {
					continue
				}
				if metric.Type() != pmetric.MetricTypeSum || metric.Sum().AggregationTemporality() != pmetric.AggregationTemporalityDelta {
					dtrp.logger.Info(fmt.Sprintf("Configured metric for rate calculation %s is not a delta sum\n", metric.Name()))
					continue
				}
				dataPointSlice := metric.Sum().DataPoints()

				for i := 0; i < dataPointSlice.Len(); i++ {
					dataPoint := dataPointSlice.At(i)

					durationNanos := time.Duration(dataPoint.Timestamp() - dataPoint.StartTimestamp())
					var rate float64
					switch dataPoint.ValueType() {
					case pmetric.NumberDataPointValueTypeDouble:
						rate = calculateRate(dataPoint.DoubleValue(), durationNanos)
					case pmetric.NumberDataPointValueTypeInt:
						rate = calculateRate(float64(dataPoint.IntValue()), durationNanos)
					default:
						return md, consumererror.NewPermanent(fmt.Errorf("invalid data point type:%d", dataPoint.ValueType()))
					}
					dataPoint.SetDoubleValue(rate)
				}

				// Setting the data type removed all the data points, so we must move them back to the metric.
				dataPointSlice.MoveAndAppendTo(metric.SetEmptyGauge().DataPoints())

				if metric.Unit() != "" {
					// Append "/s" to nonempty units to reflect the per-second rate.
					// There are a couple of edge cases that could be considered here:
					// 1. The unit could already end with "/s" or a different denominator unit.
					// 2. The unit could be a time unit which would result in a scalar.
					//
					// There are two relevant rules in the UCUM specification that apply here:
					//  a. UCUM 2.2. §7.2, https://ucum.org/ucum#section-Syntax-Rules
					//    > Terms are evaluated from left to right with the period and the solidus
					//    > having the same operator precedence. Multiple division operators are
					//    > allowed within one term."
					//  b. UCUM 2 §2.2 https://ucum.org/ucum#section-Grammar-of-Units-and-Unit-Terms
					//    > Programs that declare full conformance [...] must detect equivalence
					//    > for different expressions with the same meaning
					//
					//  This means that resulting expressions such as "m/s/s" or "ms/s" are valid UCUM
					//  units and there is no need to simplify them to be compliant.
					metric.SetUnit(metric.Unit() + "/s")
				}
			}
		}
	}
	return md, nil
}

// Shutdown is invoked during service shutdown.
func (*deltaToRateProcessor) Shutdown(context.Context) error {
	return nil
}

func calculateRate(value float64, durationNanos time.Duration) float64 {
	duration := durationNanos.Seconds()
	if duration > 0 {
		rate := value / duration
		return rate
	}
	return 0
}
