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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type deltaToRateProcessor struct {
	metrics []string
	logger  *zap.Logger
}

func newDeltaToRateProcessor(config *Config, logger *zap.Logger) *deltaToRateProcessor {
	return &deltaToRateProcessor{
		metrics: config.Metrics,
		logger:  logger,
	}
}

// Start is invoked during service startup.
func (dtrp *deltaToRateProcessor) Start(context.Context, component.Host) error {
	return nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (dtrp *deltaToRateProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	inputMetricSet := make(map[string]bool)
	for _, name := range dtrp.metrics {
		inputMetricSet[name] = true
	}

	resourceMetricsSlice := md.ResourceMetrics()

	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		ilms := rm.InstrumentationLibraryMetrics()
		for i := 0; i < ilms.Len(); i++ {
			ilm := ilms.At(i)
			metricSlice := ilm.Metrics()
			for j := 0; j < metricSlice.Len(); j++ {
				metric := metricSlice.At(j)
				_, ok := inputMetricSet[metric.Name()]
				if ok {
					newDoubleDataPointSlice := pdata.NewNumberDataPointSlice()
					if metric.DataType() == pdata.MetricDataTypeSum && metric.Sum().AggregationTemporality() == pdata.AggregationTemporalityDelta {
						dataPoints := metric.Sum().DataPoints()

						for i := 0; i < dataPoints.Len(); i++ {
							fromDataPoint := dataPoints.At(i)
							newDp := newDoubleDataPointSlice.AppendEmpty()
							fromDataPoint.CopyTo(newDp)

							durationNanos := fromDataPoint.Timestamp().AsTime().Sub(fromDataPoint.StartTimestamp().AsTime())
							durationSeconds := durationNanos.Seconds()
							if durationSeconds > 0 {
								rate := fromDataPoint.Value() / durationNanos.Seconds()
								newDp.SetValue(rate)
							}
						}
					}
					metric.SetDataType(pdata.MetricDataTypeGauge)
					for d := 0; d < newDoubleDataPointSlice.Len(); d++ {
						dp := metric.Gauge().DataPoints().AppendEmpty()
						newDoubleDataPointSlice.At(d).CopyTo(dp)
					}
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
