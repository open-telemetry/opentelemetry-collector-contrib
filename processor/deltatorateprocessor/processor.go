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
func (dtrp *deltaToRateProcessor) Start(context.Context, component.Host) error {
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
				newDoubleDataPointSlice := pmetric.NewNumberDataPointSlice()
				dataPoints := metric.Sum().DataPoints()

				for i := 0; i < dataPoints.Len(); i++ {
					fromDataPoint := dataPoints.At(i)
					newDp := newDoubleDataPointSlice.AppendEmpty()
					fromDataPoint.CopyTo(newDp)

					durationNanos := time.Duration(fromDataPoint.Timestamp() - fromDataPoint.StartTimestamp())
					var rate float64
					switch fromDataPoint.ValueType() {
					case pmetric.NumberDataPointValueTypeDouble:
						rate = calculateRate(fromDataPoint.DoubleValue(), durationNanos)
					case pmetric.NumberDataPointValueTypeInt:
						rate = calculateRate(float64(fromDataPoint.IntValue()), durationNanos)
					default:
						return md, consumererror.NewPermanent(fmt.Errorf("invalid data point type:%d", fromDataPoint.ValueType()))
					}
					newDp.SetDoubleValue(rate)
				}

				dps := metric.SetEmptyGauge().DataPoints()
				dps.EnsureCapacity(newDoubleDataPointSlice.Len())
				for d := 0; d < newDoubleDataPointSlice.Len(); d++ {
					dp := dps.AppendEmpty()
					newDoubleDataPointSlice.At(d).CopyTo(dp)
				}
			}
		}
	}
	return md, nil
}

// Shutdown is invoked during service shutdown.
func (dtrp *deltaToRateProcessor) Shutdown(context.Context) error {
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
