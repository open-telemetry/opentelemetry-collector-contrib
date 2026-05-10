// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sawmillsfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/sawmillsfuncs"

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type DataPointSlice[T DataPoint] interface {
	Len() int
	At(int) T
	AppendEmpty() T
}

type DataPoint interface {
	Attributes() pcommon.Map
}

type splitMetricArguments struct {
	Prefix              string
	PreservedAttributes ottl.Optional[[]string]
}

type generatedMetricTracker struct {
	mu      sync.Mutex
	pending map[*ottlmetric.MetricIteration]map[int]int
}

func newGeneratedMetricTracker() *generatedMetricTracker {
	return &generatedMetricTracker{
		pending: make(map[*ottlmetric.MetricIteration]map[int]int),
	}
}

func (t *generatedMetricTracker) mark(iteration *ottlmetric.MetricIteration, metricIndex int) {
	if iteration == nil || metricIndex < 0 {
		return
	}
	registerCleanup := false
	t.mu.Lock()
	pending := t.pending[iteration]
	if pending == nil {
		pending = make(map[int]int)
		t.pending[iteration] = pending
		registerCleanup = true
	}
	pending[metricIndex]++
	t.mu.Unlock()

	if registerCleanup {
		iteration.RegisterCleanup(func() {
			t.clearIteration(iteration)
		})
	}
}

func (t *generatedMetricTracker) consume(iteration *ottlmetric.MetricIteration, metricIndex int) bool {
	if iteration == nil || metricIndex < 0 {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	pending := t.pending[iteration]
	count := pending[metricIndex]
	if count == 0 {
		return false
	}
	if count == 1 {
		delete(pending, metricIndex)
		if len(pending) == 0 {
			delete(t.pending, iteration)
		}
	} else {
		pending[metricIndex] = count - 1
	}
	return true
}

func (t *generatedMetricTracker) clearIteration(iteration *ottlmetric.MetricIteration) {
	if iteration == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.pending, iteration)
}

func (t *generatedMetricTracker) pendingLen() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, pending := range t.pending {
		count += len(pending)
	}
	return count
}

func NewSplitMetricFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(
		"split_metric_by_attributes",
		&splitMetricArguments{},
		createSplitMetricFunction[K],
	)
}

func createSplitMetricFunction[K any](
	_ ottl.FunctionContext,
	oArgs ottl.Arguments,
) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*splitMetricArguments)

	if !ok {
		return nil, errors.New(
			"createSplitMetricFunction args must be of type *splitMetricArguments",
		)
	}

	return splitMetricByAttributes[K](args.Prefix, args.PreservedAttributes), nil
}

func groupNumberDataPoints(
	dps pmetric.NumberDataPointSlice,
	useStartTime bool,
) pmetric.NumberDataPointSlice {
	dpsByAttrsAndTs := make(map[string]pmetric.NumberDataPoint)
	keysInOrder := make([]string, 0, dps.Len())
	for i := 0; i < dps.Len(); i++ {
		var keyHashParts []any
		dp := dps.At(i)

		if useStartTime {
			keyHashParts = []any{dp.StartTimestamp().String()}
		}
		key := dataPointHashKey(dp.Attributes(), dp.Timestamp(), keyHashParts...)
		if mapDp, ok := dpsByAttrsAndTs[key]; !ok {
			newDp := pmetric.NewNumberDataPoint()
			dp.CopyTo(newDp)
			dpsByAttrsAndTs[key] = newDp
			keysInOrder = append(keysInOrder, key)
		} else {
			switch dp.ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				mapDp.SetDoubleValue(mapDp.DoubleValue() + dp.DoubleValue())
			case pmetric.NumberDataPointValueTypeInt:
				mapDp.SetIntValue(mapDp.IntValue() + dp.IntValue())
			}
		}
	}

	newMetrics := pmetric.NewNumberDataPointSlice()
	for _, key := range keysInOrder {
		dpsByAttrsAndTs[key].CopyTo(newMetrics.AppendEmpty())
	}

	return newMetrics
}

func dataPointHashKey(atts pcommon.Map, ts pcommon.Timestamp, other ...any) string {
	var b strings.Builder
	appendMapHash(&b, atts)
	b.WriteByte('|')
	writeHashString(&b, ts.String())
	for _, part := range other {
		b.WriteByte('|')
		writeHashAny(&b, part)
	}
	return b.String()
}

func appendMapHash(b *strings.Builder, atts pcommon.Map) {
	keys := make([]string, 0, atts.Len())
	atts.Range(func(k string, _ pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)

	b.WriteByte('{')
	for _, key := range keys {
		writeHashString(b, key)
		if val, ok := atts.Get(key); ok {
			appendValueHash(b, val)
		}
		b.WriteByte(';')
	}
	b.WriteByte('}')
}

func appendSliceHash(b *strings.Builder, slice pcommon.Slice) {
	b.WriteByte('[')
	for i := 0; i < slice.Len(); i++ {
		appendValueHash(b, slice.At(i))
		b.WriteByte(';')
	}
	b.WriteByte(']')
}

func appendValueHash(b *strings.Builder, val pcommon.Value) {
	switch val.Type() {
	case pcommon.ValueTypeEmpty:
		b.WriteByte('e')
	case pcommon.ValueTypeStr:
		b.WriteByte('s')
		writeHashString(b, val.Str())
	case pcommon.ValueTypeBool:
		b.WriteByte('b')
		writeHashString(b, strconv.FormatBool(val.Bool()))
	case pcommon.ValueTypeDouble:
		b.WriteByte('d')
		writeHashString(b, strconv.FormatFloat(val.Double(), 'g', -1, 64))
	case pcommon.ValueTypeInt:
		b.WriteByte('i')
		writeHashString(b, strconv.FormatInt(val.Int(), 10))
	case pcommon.ValueTypeMap:
		b.WriteByte('m')
		appendMapHash(b, val.Map())
	case pcommon.ValueTypeSlice:
		b.WriteByte('l')
		appendSliceHash(b, val.Slice())
	case pcommon.ValueTypeBytes:
		b.WriteByte('x')
		writeHashBytes(b, val.Bytes().AsRaw())
	default:
		b.WriteByte('u')
		writeHashString(b, fmt.Sprintf("%v", val))
	}
}

func writeHashAny(b *strings.Builder, val any) {
	switch v := val.(type) {
	case string:
		b.WriteByte('s')
		writeHashString(b, v)
	default:
		b.WriteByte('u')
		writeHashString(b, fmt.Sprintf("%#v", v))
	}
}

func writeHashString(b *strings.Builder, s string) {
	b.WriteString(strconv.Itoa(len(s)))
	b.WriteByte(':')
	b.WriteString(s)
}

func writeHashBytes(b *strings.Builder, bs []byte) {
	b.WriteString(strconv.Itoa(len(bs)))
	b.WriteByte(':')
	_, _ = b.Write(bs)
}

func findMetricIndex(metrics pmetric.MetricSlice, metric pmetric.Metric) int {
	for i := 0; i < metrics.Len(); i++ {
		if metrics.At(i) == metric {
			return i
		}
	}
	return -1
}

func splitMetricByAttributes[K any](
	prefix string,
	preservedAttrs ottl.Optional[[]string],
) ottl.ExprFunc[K] {
	preservedKeys := preservedAttrs.Get()
	preservedKeysMap := make(map[string]bool, len(preservedKeys))

	for _, key := range preservedKeys {
		preservedKeysMap[key] = true
	}

	// Track exact outputs generated by this function instead of treating every prefix match as synthetic.
	generatedMetrics := newGeneratedMetricTracker()
	generatedNamePrefix := prefix + "_"

	return func(_ context.Context, tCtx K) (any, error) {
		tmCtx, err := metricTransformContext(any(tCtx))
		if err != nil {
			return nil, err
		}

		currentMetric := tmCtx.GetMetric()
		metrics := tmCtx.GetMetrics()
		iteration, currentMetricIndex, hasMetricIteration := tmCtx.GetMetricIteration()
		newMetricName := generatedNamePrefix + currentMetric.Name()
		if !hasMetricIteration {
			iteration = ottlmetric.NewMetricIteration()
			defer iteration.Close()
			currentMetricIndex = findMetricIndex(metrics, currentMetric)
		}
		if generatedMetrics.consume(iteration, currentMetricIndex) {
			return nil, nil
		}

		switch currentMetric.Type() {
		case pmetric.MetricTypeGauge:
			gauge := currentMetric.Gauge()
			dataPoints := gauge.DataPoints()
			newDataPoints := handleDataPoints[pmetric.NumberDataPointSlice, pmetric.NumberDataPoint](
				dataPoints,
				preservedKeysMap,
				pmetric.NewNumberDataPointSlice,
				pmetric.NewNumberDataPoint,
				func(src, dest pmetric.NumberDataPoint) {
					src.CopyTo(dest)
				})
			if newDataPoints.Len() > 0 {
				newMetric := metrics.AppendEmpty()
				generatedMetrics.mark(iteration, metrics.Len()-1)
				currentMetric.CopyTo(newMetric)

				newGaugeMetric := newMetric.Gauge()
				newMetric.SetName(newMetricName)
				newDataPoints.CopyTo(newGaugeMetric.DataPoints())
			}
		case pmetric.MetricTypeSum:
			sum := currentMetric.Sum()
			dataPoints := sum.DataPoints()
			newDataPoints := handleDataPoints[pmetric.NumberDataPointSlice, pmetric.NumberDataPoint](
				dataPoints,
				preservedKeysMap,
				pmetric.NewNumberDataPointSlice,
				pmetric.NewNumberDataPoint,
				func(src, dest pmetric.NumberDataPoint) {
					src.CopyTo(dest)
				})
			if newDataPoints.Len() > 0 {
				groupByStartTime := sum.AggregationTemporality() == pmetric.AggregationTemporalityDelta
				grouped := groupNumberDataPoints(newDataPoints, groupByStartTime)
				newMetric := metrics.AppendEmpty()
				generatedMetrics.mark(iteration, metrics.Len()-1)
				currentMetric.CopyTo(newMetric)

				newSumMetric := newMetric.Sum()
				newMetric.SetName(newMetricName)
				grouped.CopyTo(newSumMetric.DataPoints())
			}
		case pmetric.MetricTypeHistogram:
			histogram := currentMetric.Histogram()
			dataPoints := histogram.DataPoints()
			newDataPoints := handleDataPoints[pmetric.HistogramDataPointSlice, pmetric.HistogramDataPoint](
				dataPoints,
				preservedKeysMap,
				pmetric.NewHistogramDataPointSlice,
				pmetric.NewHistogramDataPoint,
				func(src, dest pmetric.HistogramDataPoint) {
					src.CopyTo(dest)
				})
			if newDataPoints.Len() > 0 {
				newMetric := metrics.AppendEmpty()
				generatedMetrics.mark(iteration, metrics.Len()-1)
				currentMetric.CopyTo(newMetric)

				newHistogramMetric := newMetric.Histogram()
				newMetric.SetName(newMetricName)
				newDataPoints.CopyTo(newHistogramMetric.DataPoints())
			}
		case pmetric.MetricTypeExponentialHistogram:
			expHistogram := currentMetric.ExponentialHistogram()
			dataPoints := expHistogram.DataPoints()
			newDataPoints := handleDataPoints[pmetric.ExponentialHistogramDataPointSlice, pmetric.ExponentialHistogramDataPoint](
				dataPoints,
				preservedKeysMap,
				pmetric.NewExponentialHistogramDataPointSlice,
				pmetric.NewExponentialHistogramDataPoint,
				func(src, dest pmetric.ExponentialHistogramDataPoint) {
					src.CopyTo(dest)
				})
			if newDataPoints.Len() > 0 {
				newMetric := metrics.AppendEmpty()
				generatedMetrics.mark(iteration, metrics.Len()-1)
				currentMetric.CopyTo(newMetric)

				newHistogramMetric := newMetric.ExponentialHistogram()
				newMetric.SetName(newMetricName)
				newDataPoints.CopyTo(newHistogramMetric.DataPoints())
			}
		case pmetric.MetricTypeSummary:
			summary := currentMetric.Summary()
			dataPoints := summary.DataPoints()
			newDataPoints := handleDataPoints[pmetric.SummaryDataPointSlice, pmetric.SummaryDataPoint](
				dataPoints,
				preservedKeysMap,
				pmetric.NewSummaryDataPointSlice,
				pmetric.NewSummaryDataPoint,
				func(src, dest pmetric.SummaryDataPoint) {
					src.CopyTo(dest)
				})
			if newDataPoints.Len() > 0 {
				newMetric := metrics.AppendEmpty()
				generatedMetrics.mark(iteration, metrics.Len()-1)
				currentMetric.CopyTo(newMetric)

				newSummaryMetric := newMetric.Summary()
				newMetric.SetName(newMetricName)
				newDataPoints.CopyTo(newSummaryMetric.DataPoints())
			}
		}

		return nil, nil
	}
}

func metricTransformContext(tCtx any) (*ottlmetric.TransformContext, error) {
	switch v := tCtx.(type) {
	case *ottlmetric.TransformContext:
		return v, nil
	default:
		return nil, errors.New("split_metric_by_attributes func can run only inside metric statement")
	}
}

func handleDataPoints[S DataPointSlice[P], P DataPoint](
	dataPoints S,
	preservedKeysMap map[string]bool,
	newDataPointSliceFunc func() S,
	newDataPointFunc func() P,
	copyFunc func(src, dest P),
) S {
	newDataPoints := newDataPointSliceFunc()

	for i := 0; i < dataPoints.Len(); i++ {
		point := dataPoints.At(i)
		attrs := point.Attributes()

		if attrs.Len() <= 1 {
			copyFunc(point, newDataPoints.AppendEmpty())
			continue
		}

		varyingAttrs := make(map[string]pcommon.Value)
		attrs.Range(func(k string, v pcommon.Value) bool {
			if !preservedKeysMap[k] {
				varyingAttrs[k] = v
			}
			return true
		})

		if len(varyingAttrs) <= 1 {
			copyFunc(point, newDataPoints.AppendEmpty())
			continue
		}

		for varyingAttrKey := range varyingAttrs {
			newPoint := newDataPointFunc()
			copyFunc(point, newPoint)

			newPoint.Attributes().RemoveIf(func(k string, _ pcommon.Value) bool {
				if preservedKeysMap[k] || k == varyingAttrKey {
					return false
				}
				return true
			})

			copyFunc(newPoint, newDataPoints.AppendEmpty())
		}
	}

	return newDataPoints
}
