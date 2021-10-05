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

package cumulativetodeltaprocessor

import (
	"context"
	"math"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

type cumulativeToDeltaProcessor struct {
	metrics         map[string]bool
	logger          *zap.Logger
	deltaCalculator awsmetrics.MetricCalculator
}

func newCumulativeToDeltaProcessor(config *Config, logger *zap.Logger) *cumulativeToDeltaProcessor {
	inputMetricSet := make(map[string]bool, len(config.Metrics))
	for _, name := range config.Metrics {
		inputMetricSet[name] = true
	}

	return &cumulativeToDeltaProcessor{
		metrics:         inputMetricSet,
		logger:          logger,
		deltaCalculator: newDeltaCalculator(),
	}
}

// Start is invoked during service startup.
func (ctdp *cumulativeToDeltaProcessor) Start(context.Context, component.Host) error {
	return nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (ctdp *cumulativeToDeltaProcessor) processMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	resourceMetricsSlice := md.ResourceMetrics()
	for i := 0; i < resourceMetricsSlice.Len(); i++ {
		rm := resourceMetricsSlice.At(i)
		ilms := rm.InstrumentationLibraryMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ilm := ilms.At(j)
			metricSlice := ilm.Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				metric := metricSlice.At(k)
				if ctdp.metrics[metric.Name()] {
					if metric.DataType() == pdata.MetricDataTypeSum && metric.Sum().AggregationTemporality() == pdata.MetricAggregationTemporalityCumulative {
						dataPoints := metric.Sum().DataPoints()

						for l := 0; l < dataPoints.Len(); l++ {
							fromDataPoint := dataPoints.At(l)
							labelMap := make(map[string]string)

							fromDataPoint.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
								labelMap[k] = v.AsString()
								return true
							})
							datapointValue := fromDataPoint.DoubleVal()
							if math.IsNaN(datapointValue) {
								continue
							}
							result, _ := ctdp.deltaCalculator.Calculate(metric.Name(), labelMap, datapointValue, fromDataPoint.Timestamp().AsTime())

							fromDataPoint.SetDoubleVal(result.(delta).value)
							fromDataPoint.SetStartTimestamp(pdata.NewTimestampFromTime(result.(delta).prevTimestamp))
						}
						metric.Sum().SetAggregationTemporality(pdata.MetricAggregationTemporalityDelta)
					}
				}
			}
		}
	}
	return md, nil
}

// Shutdown is invoked during service shutdown.
func (ctdp *cumulativeToDeltaProcessor) Shutdown(context.Context) error {
	return nil
}

func newDeltaCalculator() awsmetrics.MetricCalculator {
	return awsmetrics.NewMetricCalculator(func(prev *awsmetrics.MetricValue, val interface{}, timestamp time.Time) (interface{}, bool) {
		result := delta{value: val.(float64), prevTimestamp: timestamp}

		if prev != nil {
			deltaValue := val.(float64) - prev.RawValue.(float64)
			result.value = deltaValue
			result.prevTimestamp = prev.Timestamp
			return result, true
		}
		return result, false
	})
}

type delta struct {
	value         float64
	prevTimestamp time.Time
}
