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

package deltatorateprocessor

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
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
func (dtrp *deltaToRateProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	resourceMetricsSlice := md.ResourceMetrics()

	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		ilms := rm.InstrumentationLibraryMetrics()
		for i := 0; i < ilms.Len(); i++ {
			ilm := ilms.At(i)
			metricSlice := ilm.Metrics()
			for j := 0; j < metricSlice.Len(); j++ {
				metric := metricSlice.At(j)
				if _, ok := dtrp.ConfiguredMetrics[metric.Name()]; !ok {
					continue
				}
				if metric.DataType() != pdata.MetricDataTypeSum || metric.Sum().AggregationTemporality() != pdata.MetricAggregationTemporalityDelta {
					dtrp.logger.Info(fmt.Sprintf("Configured metric for rate calculation %s is not a delta sum\n", metric.Name()))
					continue
				}
				newDoubleDataPointSlice := pdata.NewNumberDataPointSlice()
				dataPoints := metric.Sum().DataPoints()

				for i := 0; i < dataPoints.Len(); i++ {
					fromDataPoint := dataPoints.At(i)
					newDp := newDoubleDataPointSlice.AppendEmpty()
					fromDataPoint.CopyTo(newDp)

					durationNanos := time.Duration(fromDataPoint.Timestamp() - fromDataPoint.StartTimestamp())
					rate := calculateRate(fromDataPoint.DoubleVal(), durationNanos)
					newDp.SetDoubleVal(rate)
				}

				metric.SetDataType(pdata.MetricDataTypeGauge)
				for d := 0; d < newDoubleDataPointSlice.Len(); d++ {
					dp := metric.Gauge().DataPoints().AppendEmpty()
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
