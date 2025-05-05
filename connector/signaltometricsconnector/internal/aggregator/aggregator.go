// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregator // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/aggregator"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// Aggregator provides a single interface to update all metrics
// datastructures. The required datastructure is selected using
// the metric definition.
type Aggregator[K any] struct {
	result pmetric.Metrics
	// smLookup maps resourceID against scope metrics since the aggregator
	// always produces a single scope.
	smLookup    map[[16]byte]pmetric.ScopeMetrics
	valueCounts map[model.MetricKey]map[[16]byte]map[[16]byte]*valueCountDP
	sums        map[model.MetricKey]map[[16]byte]map[[16]byte]*sumDP
	timestamp   time.Time
}

// NewAggregator creates a new instance of aggregator.
func NewAggregator[K any](metrics pmetric.Metrics) *Aggregator[K] {
	return &Aggregator[K]{
		result:      metrics,
		smLookup:    make(map[[16]byte]pmetric.ScopeMetrics),
		valueCounts: make(map[model.MetricKey]map[[16]byte]map[[16]byte]*valueCountDP),
		sums:        make(map[model.MetricKey]map[[16]byte]map[[16]byte]*sumDP),
		timestamp:   time.Now(),
	}
}

func (a *Aggregator[K]) Aggregate(
	ctx context.Context,
	tCtx K,
	md model.MetricDef[K],
	resAttrs, srcAttrs pcommon.Map,
	defaultCount int64,
) error {
	switch md.Key.Type {
	case pmetric.MetricTypeExponentialHistogram:
		val, count, err := getValueCount(
			ctx, tCtx,
			md.ExponentialHistogram.Value,
			md.ExponentialHistogram.Count,
			defaultCount,
		)
		if err != nil {
			return err
		}
		return a.aggregateValueCount(md, resAttrs, srcAttrs, val, count)
	case pmetric.MetricTypeHistogram:
		val, count, err := getValueCount(
			ctx, tCtx,
			md.ExplicitHistogram.Value,
			md.ExplicitHistogram.Count,
			defaultCount,
		)
		if err != nil {
			return err
		}
		return a.aggregateValueCount(md, resAttrs, srcAttrs, val, count)
	case pmetric.MetricTypeSum:
		raw, err := md.Sum.Value.Eval(ctx, tCtx)
		if err != nil {
			return fmt.Errorf("failed to execute OTTL value for sum: %w", err)
		}
		switch v := raw.(type) {
		case int64:
			return a.aggregateInt(md, resAttrs, srcAttrs, v)
		case float64:
			return a.aggregateDouble(md, resAttrs, srcAttrs, v)
		default:
			return fmt.Errorf(
				"failed to parse sum OTTL value of type %T into int64 or float64: %v",
				v, v,
			)
		}
	}
	return nil
}

// Finalize finalizes the aggregations performed by the aggregator so far into
// the pmetric.Metrics used to create this instance of the aggregator. Finalize
// should be called once per aggregator instance and the aggregator instance
// should not be used after Finalize is called.
func (a *Aggregator[K]) Finalize(mds []model.MetricDef[K]) {
	for _, md := range mds {
		for resID, dpMap := range a.valueCounts[md.Key] {
			metrics := a.smLookup[resID].Metrics()
			var (
				destExpHist      pmetric.ExponentialHistogram
				destExplicitHist pmetric.Histogram
			)
			switch md.Key.Type {
			case pmetric.MetricTypeExponentialHistogram:
				destMetric := metrics.AppendEmpty()
				destMetric.SetName(md.Key.Name)
				destMetric.SetUnit(md.Key.Unit)
				destMetric.SetDescription(md.Key.Description)
				destExpHist = destMetric.SetEmptyExponentialHistogram()
				destExpHist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				destExpHist.DataPoints().EnsureCapacity(len(dpMap))
			case pmetric.MetricTypeHistogram:
				destMetric := metrics.AppendEmpty()
				destMetric.SetName(md.Key.Name)
				destMetric.SetUnit(md.Key.Unit)
				destMetric.SetDescription(md.Key.Description)
				destExplicitHist = destMetric.SetEmptyHistogram()
				destExplicitHist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				destExplicitHist.DataPoints().EnsureCapacity(len(dpMap))
			}
			for _, dp := range dpMap {
				dp.Copy(
					a.timestamp,
					destExpHist,
					destExplicitHist,
				)
			}
		}
		for resID, dpMap := range a.sums[md.Key] {
			if md.Sum == nil {
				continue
			}
			metrics := a.smLookup[resID].Metrics()
			destMetric := metrics.AppendEmpty()
			destMetric.SetName(md.Key.Name)
			destMetric.SetUnit(md.Key.Unit)
			destMetric.SetDescription(md.Key.Description)
			destCounter := destMetric.SetEmptySum()
			destCounter.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			destCounter.DataPoints().EnsureCapacity(len(dpMap))
			for _, dp := range dpMap {
				dp.Copy(a.timestamp, destCounter.DataPoints().AppendEmpty())
			}
		}
		// If there are two metric defined with the same key required by metricKey
		// then they will be aggregated within the same metric and produced
		// together. Deleting the key ensures this while preventing duplicates.
		delete(a.valueCounts, md.Key)
		delete(a.sums, md.Key)
	}
}

func (a *Aggregator[K]) aggregateInt(
	md model.MetricDef[K],
	resAttrs, srcAttrs pcommon.Map,
	v int64,
) error {
	resID := a.getResourceID(resAttrs)
	attrID := pdatautil.MapHash(srcAttrs)
	if _, ok := a.sums[md.Key]; !ok {
		a.sums[md.Key] = make(map[[16]byte]map[[16]byte]*sumDP)
	}
	if _, ok := a.sums[md.Key][resID]; !ok {
		a.sums[md.Key][resID] = make(map[[16]byte]*sumDP)
	}
	if _, ok := a.sums[md.Key][resID][attrID]; !ok {
		a.sums[md.Key][resID][attrID] = newSumDP(srcAttrs, false)
	}
	a.sums[md.Key][resID][attrID].AggregateInt(v)
	return nil
}

func (a *Aggregator[K]) aggregateDouble(
	md model.MetricDef[K],
	resAttrs, srcAttrs pcommon.Map,
	v float64,
) error {
	resID := a.getResourceID(resAttrs)
	attrID := pdatautil.MapHash(srcAttrs)
	if _, ok := a.sums[md.Key]; !ok {
		a.sums[md.Key] = make(map[[16]byte]map[[16]byte]*sumDP)
	}
	if _, ok := a.sums[md.Key][resID]; !ok {
		a.sums[md.Key][resID] = make(map[[16]byte]*sumDP)
	}
	if _, ok := a.sums[md.Key][resID][attrID]; !ok {
		a.sums[md.Key][resID][attrID] = newSumDP(srcAttrs, true)
	}
	a.sums[md.Key][resID][attrID].AggregateDouble(v)
	return nil
}

func (a *Aggregator[K]) aggregateValueCount(
	md model.MetricDef[K],
	resAttrs, srcAttrs pcommon.Map,
	value float64, count int64,
) error {
	if count == 0 {
		// Nothing to record as count is zero
		return nil
	}
	resID := a.getResourceID(resAttrs)
	attrID := pdatautil.MapHash(srcAttrs)
	if _, ok := a.valueCounts[md.Key]; !ok {
		a.valueCounts[md.Key] = make(map[[16]byte]map[[16]byte]*valueCountDP)
	}
	if _, ok := a.valueCounts[md.Key][resID]; !ok {
		a.valueCounts[md.Key][resID] = make(map[[16]byte]*valueCountDP)
	}
	if _, ok := a.valueCounts[md.Key][resID][attrID]; !ok {
		a.valueCounts[md.Key][resID][attrID] = newValueCountDP(md, srcAttrs)
	}
	a.valueCounts[md.Key][resID][attrID].Aggregate(value, count)
	return nil
}

func (a *Aggregator[K]) getResourceID(resourceAttrs pcommon.Map) [16]byte {
	resID := pdatautil.MapHash(resourceAttrs)
	if _, ok := a.smLookup[resID]; !ok {
		destResourceMetric := a.result.ResourceMetrics().AppendEmpty()
		destResAttrs := destResourceMetric.Resource().Attributes()
		destResAttrs.EnsureCapacity(resourceAttrs.Len() + 1)
		resourceAttrs.CopyTo(destResAttrs)
		destScopeMetric := destResourceMetric.ScopeMetrics().AppendEmpty()
		destScopeMetric.Scope().SetName(metadata.ScopeName)
		a.smLookup[resID] = destScopeMetric
	}
	return resID
}

// getValueCount evaluates OTTL to get count and value respectively. Count is
// optional and defaults to the default count if the OTTL statement for count
// is missing. Value is required and returns an error if OTTL statement for
// value is missing.
func getValueCount[K any](
	ctx context.Context, tCtx K,
	valueExpr, countExpr *ottl.ValueExpression[K],
	defaultCount int64,
) (float64, int64, error) {
	val, err := getDoubleFromOTTL(ctx, tCtx, valueExpr)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get value from OTTL: %w", err)
	}
	count := defaultCount
	if countExpr != nil {
		count, err = getIntFromOTTL(ctx, tCtx, countExpr)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to get count from OTTL: %w", err)
		}
	}
	return val, count, nil
}

func getIntFromOTTL[K any](
	ctx context.Context,
	tCtx K,
	s *ottl.ValueExpression[K],
) (int64, error) {
	if s == nil {
		return 0, nil
	}
	raw, err := s.Eval(ctx, tCtx)
	if err != nil {
		return 0, err
	}
	switch v := raw.(type) {
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	default:
		return 0, fmt.Errorf(
			"failed to parse int OTTL value, expression returned value of type %T: %v",
			v, v,
		)
	}
}

func getDoubleFromOTTL[K any](
	ctx context.Context,
	tCtx K,
	s *ottl.ValueExpression[K],
) (float64, error) {
	if s == nil {
		return 0, nil
	}
	raw, err := s.Eval(ctx, tCtx)
	if err != nil {
		return 0, err
	}
	switch v := raw.(type) {
	case float64:
		return v, nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf(
			"failed to parse double OTTL value, expression returned value of type %T: %v",
			v, v,
		)
	}
}
