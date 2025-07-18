// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"

import (
	"context"
	"fmt"
	"math"
	"strings"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"
)

type cumulativeToDeltaProcessor struct {
	includeFS          filterset.FilterSet
	excludeFS          filterset.FilterSet
	includeMetricTypes map[pmetric.MetricType]bool
	excludeMetricTypes map[pmetric.MetricType]bool
	logger             *zap.Logger
	deltaCalculator    *tracking.MetricTracker
	cancelFunc         context.CancelFunc
}

func newCumulativeToDeltaProcessor(config *Config, logger *zap.Logger) (*cumulativeToDeltaProcessor, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &cumulativeToDeltaProcessor{
		logger:     logger,
		cancelFunc: cancel,
	}
	if len(config.Include.Metrics) > 0 {
		p.includeFS, _ = filterset.CreateFilterSet(config.Include.Metrics, &config.Include.Config)
	}
	if len(config.Exclude.Metrics) > 0 {
		p.excludeFS, _ = filterset.CreateFilterSet(config.Exclude.Metrics, &config.Exclude.Config)
	}

	if len(config.Include.MetricTypes) > 0 {
		includeMetricTypeFilter, err := getMetricTypeFilter(config.Include.MetricTypes)
		if err != nil {
			return nil, err
		}
		p.includeMetricTypes = includeMetricTypeFilter
	}

	if len(config.Exclude.MetricTypes) > 0 {
		excludeMetricTypeFilter, err := getMetricTypeFilter(config.Exclude.MetricTypes)
		if err != nil {
			return nil, err
		}
		p.excludeMetricTypes = excludeMetricTypeFilter
	}

	p.deltaCalculator = tracking.NewMetricTracker(ctx, logger, config.MaxStaleness, config.InitialValue)

	return p, nil
}

func getMetricTypeFilter(types []string) (map[pmetric.MetricType]bool, error) {
	res := map[pmetric.MetricType]bool{}
	for _, t := range types {
		switch strings.ToLower(t) {
		case strings.ToLower(pmetric.MetricTypeSum.String()):
			res[pmetric.MetricTypeSum] = true
		case strings.ToLower(pmetric.MetricTypeHistogram.String()):
			res[pmetric.MetricTypeHistogram] = true
		default:
			return nil, fmt.Errorf("unsupported metric type filter: %s", t)
		}
	}
	return res, nil
}

// processMetrics implements the ProcessMetricsFunc type.
func (ctdp *cumulativeToDeltaProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	md.ResourceMetrics().RemoveIf(func(rm pmetric.ResourceMetrics) bool {
		rm.ScopeMetrics().RemoveIf(func(ilm pmetric.ScopeMetrics) bool {
			ilm.Metrics().RemoveIf(func(m pmetric.Metric) bool {
				if !ctdp.shouldConvertMetric(m) {
					return false
				}
				switch m.Type() {
				case pmetric.MetricTypeSum:
					ms := m.Sum()
					if ms.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					// Ignore any metrics that aren't monotonic
					if !ms.IsMonotonic() {
						return false
					}

					baseIdentity := tracking.MetricIdentity{
						Resource:               rm.Resource(),
						InstrumentationLibrary: ilm.Scope(),
						MetricType:             m.Type(),
						MetricName:             m.Name(),
						MetricUnit:             m.Unit(),
						MetricIsMonotonic:      ms.IsMonotonic(),
					}
					ctdp.convertNumberDataPoints(ms.DataPoints(), baseIdentity)
					ms.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					return ms.DataPoints().Len() == 0
				case pmetric.MetricTypeHistogram:
					ms := m.Histogram()
					if ms.AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						return false
					}

					if ms.DataPoints().Len() == 0 {
						return false
					}

					baseIdentity := tracking.MetricIdentity{
						Resource:               rm.Resource(),
						InstrumentationLibrary: ilm.Scope(),
						MetricType:             m.Type(),
						MetricName:             m.Name(),
						MetricUnit:             m.Unit(),
						MetricIsMonotonic:      true,
						MetricValueType:        pmetric.NumberDataPointValueTypeInt,
					}

					ctdp.convertHistogramDataPoints(ms.DataPoints(), baseIdentity)

					ms.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
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

func (ctdp *cumulativeToDeltaProcessor) shutdown(context.Context) error {
	ctdp.cancelFunc()
	return nil
}

func (ctdp *cumulativeToDeltaProcessor) shouldConvertMetric(metric pmetric.Metric) bool {
	return (ctdp.includeFS == nil || ctdp.includeFS.Matches(metric.Name())) &&
		(len(ctdp.includeMetricTypes) == 0 || ctdp.includeMetricTypes[metric.Type()]) &&
		(ctdp.excludeFS == nil || !ctdp.excludeFS.Matches(metric.Name())) &&
		(len(ctdp.excludeMetricTypes) == 0 || !ctdp.excludeMetricTypes[metric.Type()])
}

func (ctdp *cumulativeToDeltaProcessor) convertNumberDataPoints(dps pmetric.NumberDataPointSlice, baseIdentity tracking.MetricIdentity) {
	dps.RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		id := baseIdentity
		id.StartTimestamp = dp.StartTimestamp()
		id.Attributes = dp.Attributes()
		id.MetricValueType = dp.ValueType()
		point := tracking.ValuePoint{
			ObservedTimestamp: dp.Timestamp(),
		}

		if dp.Flags().NoRecordedValue() {
			// drop points with no value
			return true
		}
		if id.IsFloatVal() {
			// Do not attempt to transform NaN values
			if math.IsNaN(dp.DoubleValue()) {
				return false
			}
			point.FloatValue = dp.DoubleValue()
		} else {
			point.IntValue = dp.IntValue()
		}
		trackingPoint := tracking.MetricPoint{
			Identity: id,
			Value:    point,
		}
		delta, valid := ctdp.deltaCalculator.Convert(trackingPoint)
		if !valid {
			return true
		}
		dp.SetStartTimestamp(delta.StartTimestamp)
		if id.IsFloatVal() {
			dp.SetDoubleValue(delta.FloatValue)
		} else {
			dp.SetIntValue(delta.IntValue)
		}
		return false
	})
}

func (ctdp *cumulativeToDeltaProcessor) convertHistogramDataPoints(in any, baseIdentity tracking.MetricIdentity) {
	if dps, ok := in.(pmetric.HistogramDataPointSlice); ok {
		dps.RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
			id := baseIdentity
			id.StartTimestamp = dp.StartTimestamp()
			id.Attributes = dp.Attributes()

			if dp.Flags().NoRecordedValue() {
				// drop points with no value
				return true
			}

			point := tracking.ValuePoint{
				ObservedTimestamp: dp.Timestamp(),
				HistogramValue: &tracking.HistogramPoint{
					Count:   dp.Count(),
					Sum:     dp.Sum(),
					Buckets: dp.BucketCounts().AsRaw(),
				},
			}

			trackingPoint := tracking.MetricPoint{
				Identity: id,
				Value:    point,
			}
			delta, valid := ctdp.deltaCalculator.Convert(trackingPoint)

			if valid {
				dp.SetStartTimestamp(delta.StartTimestamp)
				dp.SetCount(delta.HistogramValue.Count)
				if dp.HasSum() && !math.IsNaN(dp.Sum()) {
					dp.SetSum(delta.HistogramValue.Sum)
				}
				dp.BucketCounts().FromRaw(delta.HistogramValue.Buckets)
				dp.RemoveMin()
				dp.RemoveMax()
				return false
			}

			return !valid
		})
	}
}
