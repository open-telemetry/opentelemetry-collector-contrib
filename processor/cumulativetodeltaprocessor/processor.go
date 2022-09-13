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

package cumulativetodeltaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"

import (
	"context"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/service/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"
)

const enableHistogramSupportGateID = "processor.cumulativetodeltaprocessor.EnableHistogramSupport"

var enableHistogramSupportGate = featuregate.Gate{
	ID:          enableHistogramSupportGateID,
	Enabled:     false,
	Description: "wip",
}

func init() {
	featuregate.GetRegistry().MustRegister(enableHistogramSupportGate)
}

type cumulativeToDeltaProcessor struct {
	includeFS               filterset.FilterSet
	excludeFS               filterset.FilterSet
	logger                  *zap.Logger
	deltaCalculator         *tracking.MetricTracker
	cancelFunc              context.CancelFunc
	histogramSupportEnabled bool
}

func newCumulativeToDeltaProcessor(config *Config, logger *zap.Logger) *cumulativeToDeltaProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	p := &cumulativeToDeltaProcessor{
		logger:                  logger,
		deltaCalculator:         tracking.NewMetricTracker(ctx, logger, config.MaxStaleness),
		cancelFunc:              cancel,
		histogramSupportEnabled: featuregate.GetRegistry().IsEnabled(enableHistogramSupportGateID),
	}
	if len(config.Include.Metrics) > 0 {
		p.includeFS, _ = filterset.CreateFilterSet(config.Include.Metrics, &config.Include.Config)
	}
	if len(config.Exclude.Metrics) > 0 {
		p.excludeFS, _ = filterset.CreateFilterSet(config.Exclude.Metrics, &config.Exclude.Config)
	}
	return p
}

// processMetrics implements the ProcessMetricsFunc type.
func (ctdp *cumulativeToDeltaProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	resourceMetricsSlice := md.ResourceMetrics()
	resourceMetricsSlice.RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		ilms := rm.ScopeMetrics()
		ilms.RemoveIf(func(ilm pmetric.ScopeMetrics) bool {
			ms := ilm.Metrics()
			ms.RemoveIf(func(m pmetric.Metric) bool {
				if !ctdp.shouldConvertMetric(m.Name()) {
					return false
				}
				switch m.DataType() {
				case pmetric.MetricDataTypeSum:
					ms := m.Sum()
					if ms.AggregationTemporality() != pmetric.MetricAggregationTemporalityCumulative {
						return false
					}

					// Ignore any metrics that aren't monotonic
					if !ms.IsMonotonic() {
						return false
					}

					baseIdentity := tracking.MetricIdentity{
						Resource:               rm.Resource(),
						InstrumentationLibrary: ilm.Scope(),
						MetricDataType:         m.DataType(),
						MetricName:             m.Name(),
						MetricUnit:             m.Unit(),
						MetricIsMonotonic:      ms.IsMonotonic(),
					}
					ctdp.convertDataPoints(ms.DataPoints(), baseIdentity)
					ms.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
					return ms.DataPoints().Len() == 0
				case pmetric.MetricDataTypeHistogram:
					if !ctdp.histogramSupportEnabled {
						return false
					}

					ms := m.Histogram()
					if ms.AggregationTemporality() != pmetric.MetricAggregationTemporalityCumulative {
						return false
					}

					if ms.DataPoints().Len() == 0 {
						return false
					}

					countIdentity := tracking.MetricIdentity{
						Resource:               rm.Resource(),
						InstrumentationLibrary: ilm.Scope(),
						MetricDataType:         m.DataType(),
						MetricName:             m.Name(),
						MetricUnit:             m.Unit(),
						MetricIsMonotonic:      true,
						MetricValueType:        pmetric.NumberDataPointValueTypeInt,
						MetricField:            "count",
					}

					sumIdentity := countIdentity
					sumIdentity.MetricField = "sum"
					sumIdentity.MetricValueType = pmetric.NumberDataPointValueTypeDouble

					bucketIdentities := makeBucketIdentities(countIdentity, ms.DataPoints().At(0))

					histogramIdentities := tracking.HistogramIdentities{
						CountIdentity:    countIdentity,
						SumIdentity:      sumIdentity,
						BucketIdentities: bucketIdentities,
					}

					ctdp.convertHistogramDataPoints(ms.DataPoints(), &histogramIdentities)

					ms.SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
					return ms.DataPoints().Len() == 0
				default:
					return false
				}
			})
			return ilm.Metrics().Len() == 0
		})
		return rm.ScopeMetrics().Len() == 0
	})
	return md, nil
}

func makeBucketIdentities(baseIdentity tracking.MetricIdentity, dp pmetric.HistogramDataPoint) []tracking.MetricIdentity {
	numBuckets := dp.BucketCounts().Len()
	bucketIdentities := make([]tracking.MetricIdentity, numBuckets)

	for index := 0; index < numBuckets; index++ {
		bucketIdentity := baseIdentity
		bucketIdentity.MetricField = fmt.Sprintf("bucket_%d", index)
		bucketIdentities[index] = bucketIdentity
	}

	return bucketIdentities
}

func (ctdp *cumulativeToDeltaProcessor) shutdown(context.Context) error {
	ctdp.cancelFunc()
	return nil
}

func (ctdp *cumulativeToDeltaProcessor) shouldConvertMetric(metricName string) bool {
	return (ctdp.includeFS == nil || ctdp.includeFS.Matches(metricName)) &&
		(ctdp.excludeFS == nil || !ctdp.excludeFS.Matches(metricName))
}

func (ctdp *cumulativeToDeltaProcessor) convertHistogramFloatValue(id tracking.MetricIdentity, dp pmetric.HistogramDataPoint, value float64) (tracking.DeltaValue, bool) {
	id.StartTimestamp = dp.StartTimestamp()
	id.Attributes = dp.Attributes()
	trackingPoint := tracking.MetricPoint{
		Identity: id,
		Value: tracking.ValuePoint{
			ObservedTimestamp: dp.Timestamp(),
			FloatValue:        value,
		},
	}
	return ctdp.deltaCalculator.Convert(trackingPoint)
}

func (ctdp *cumulativeToDeltaProcessor) convertHistogramIntValue(id tracking.MetricIdentity, dp pmetric.HistogramDataPoint, value int64) (tracking.DeltaValue, bool) {
	id.StartTimestamp = dp.StartTimestamp()
	id.Attributes = dp.Attributes()
	trackingPoint := tracking.MetricPoint{
		Identity: id,
		Value: tracking.ValuePoint{
			ObservedTimestamp: dp.Timestamp(),
			IntValue:          value,
		},
	}
	return ctdp.deltaCalculator.Convert(trackingPoint)
}

func (ctdp *cumulativeToDeltaProcessor) convertDataPoints(in interface{}, baseIdentity tracking.MetricIdentity) {

	if dps, ok := in.(pmetric.NumberDataPointSlice); ok {
		dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
			id := baseIdentity
			id.StartTimestamp = dp.StartTimestamp()
			id.Attributes = dp.Attributes()
			id.MetricValueType = dp.ValueType()
			point := tracking.ValuePoint{
				ObservedTimestamp: dp.Timestamp(),
			}
			if id.IsFloatVal() {
				// Do not attempt to transform NaN values
				if math.IsNaN(dp.DoubleVal()) {
					return false
				}
				point.FloatValue = dp.DoubleVal()
			} else {
				point.IntValue = dp.IntVal()
			}
			trackingPoint := tracking.MetricPoint{
				Identity: id,
				Value:    point,
			}
			delta, valid := ctdp.deltaCalculator.Convert(trackingPoint)

			// When converting non-monotonic cumulative counters,
			// the first data point is omitted since the initial
			// reference is not assumed to be zero
			if !valid {
				return true
			}
			dp.SetStartTimestamp(delta.StartTimestamp)
			if id.IsFloatVal() {
				dp.SetDoubleVal(delta.FloatValue)
			} else {
				dp.SetIntVal(delta.IntValue)
			}
			return false
		})
	}
}

func (ctdp *cumulativeToDeltaProcessor) convertHistogramDataPoints(in interface{}, baseIdentities *tracking.HistogramIdentities) {

	if dps, ok := in.(pmetric.HistogramDataPointSlice); ok {
		dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
			countID := baseIdentities.CountIdentity
			countDelta, countValid := ctdp.convertHistogramIntValue(countID, dp, int64(dp.Count()))

			hasSum := dp.HasSum() && !math.IsNaN(dp.Sum())
			sumDelta, sumValid := tracking.DeltaValue{}, true

			if hasSum {
				sumID := baseIdentities.SumIdentity
				sumDelta, sumValid = ctdp.convertHistogramFloatValue(sumID, dp, dp.Sum())
			}

			bucketsValid := true
			rawBucketCounts := dp.BucketCounts().AsRaw()
			for index := 0; index < len(rawBucketCounts); index++ {
				bucketID := baseIdentities.BucketIdentities[index]
				bucketDelta, bucketValid := ctdp.convertHistogramIntValue(bucketID, dp, int64(rawBucketCounts[index]))
				rawBucketCounts[index] = uint64(bucketDelta.IntValue)
				bucketsValid = bucketsValid && bucketValid
			}

			if countValid && sumValid && bucketsValid {
				dp.SetStartTimestamp(countDelta.StartTimestamp)
				dp.SetCount(uint64(countDelta.IntValue))
				if hasSum {
					dp.SetSum(sumDelta.FloatValue)
				}
				dp.BucketCounts().FromRaw(rawBucketCounts)
				return false
			}

			return true
		})
	}
}
