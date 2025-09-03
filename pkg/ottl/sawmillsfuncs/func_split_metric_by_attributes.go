package sawmillsfuncs

import (
	"context"
	"errors"
	"fmt"

	jsoniter "github.com/json-iterator/go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type DataPointSlice[T DataPoint] interface {
	Len() int
	At(int) T
	AppendEmpty() T
	EnsureCapacity(int)
	RemoveIf(func(T) bool)
}

type DataPoint interface {
	Attributes() pcommon.Map
}

type splitMetricArguments struct {
	Prefix              string                  // New metric name prefix
	PreservedAttributes ottl.Optional[[]string] // List of attribute keys to preserve in all split metrics
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

	return splitMetricByAttributes[K](args.Prefix, args.PreservedAttributes)
}

func groupNumberDataPoints(
	dps pmetric.NumberDataPointSlice,
	useStartTime bool,
) pmetric.NumberDataPointSlice {
	dpsByAttrsAndTs := make(map[string]pmetric.NumberDataPoint)
	for i := 0; i < dps.Len(); i++ {
		var keyHashParts []any
		dp := dps.At(i)

		if useStartTime {
			keyHashParts = []any{dp.StartTimestamp().String()}
		}
		key := dataPointHashKey(dp.Attributes(), dp.Timestamp(), keyHashParts...)
		if mapDp, ok := dpsByAttrsAndTs[key]; !ok {
			dpsByAttrsAndTs[key] = dp
		} else {
			switch dp.ValueType() {
			case pmetric.NumberDataPointValueTypeDouble:
				dp.SetDoubleValue(dp.DoubleValue() + mapDp.DoubleValue())
			case pmetric.NumberDataPointValueTypeInt:
				dp.SetIntValue(dp.IntValue() + mapDp.IntValue())
			}
			dpsByAttrsAndTs[key] = dp
		}
	}

	newMetrics := pmetric.NewNumberDataPointSlice()
	for _, dp := range dpsByAttrsAndTs {
		dp.CopyTo(newMetrics.AppendEmpty())
	}

	return newMetrics
}

func dataPointHashKey(atts pcommon.Map, ts pcommon.Timestamp, other ...any) string {
	hashParts := []any{atts.AsRaw(), ts.String()}
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	jsonStr, _ := json.Marshal(append(hashParts, other...))
	return string(jsonStr)
}

func splitMetricByAttributes[K any](
	prefix string,
	preservedAttrs ottl.Optional[[]string],
) (ottl.ExprFunc[K], error) {
	preservedKeys := preservedAttrs.Get()
	preservedKeysMap := make(map[string]bool, len(preservedKeys))

	for _, key := range preservedKeys {
		preservedKeysMap[key] = true
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		if tmCtx, ok := any(tCtx).(ottlmetric.TransformContext); !ok {
			return nil, errors.New(
				"split_metric_by_attributes func can run only inside metric statement",
			)
		} else {
			currentMetric := tmCtx.GetMetric()
			metrics := tmCtx.GetMetrics()
			newMetricName := fmt.Sprintf("%s_%s", prefix, currentMetric.Name())

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
					grouped := groupNumberDataPoints(newDataPoints, false)
					newMetric := metrics.AppendEmpty()
					currentMetric.CopyTo(newMetric)

					newGaugeMetric := newMetric.Gauge()
					newMetric.SetName(newMetricName)
					grouped.CopyTo(newGaugeMetric.DataPoints())
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
					currentMetric.CopyTo(newMetric)

					newSummaryMetric := newMetric.Summary()
					newMetric.SetName(newMetricName)
					newDataPoints.CopyTo(newSummaryMetric.DataPoints())
				}
			}

			return nil, nil
		}
	}, nil
}

func handleDataPoints[S DataPointSlice[P], P DataPoint](
	dataPoints S,
	preservedKeysMap map[string]bool,
	newDataPointSliceFunc func() S,
	newDataPointFunc func() P,
	copyFunc func(src P, dest P),
) S {
	newDataPoints := newDataPointSliceFunc()

	dataPoints.RemoveIf(func(point P) bool {
		attrs := point.Attributes()

		if attrs.Len() <= 1 {
			copyFunc(point, newDataPoints.AppendEmpty())
			return false
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
			return false
		}

		for varyingAttrKey := range varyingAttrs {
			newPoint := newDataPointFunc()
			copyFunc(point, newPoint)

			newPoint.Attributes().RemoveIf(func(k string, v pcommon.Value) bool {
				if preservedKeysMap[k] || k == varyingAttrKey {
					return false
				}
				return true
			})

			copyFunc(newPoint, newDataPoints.AppendEmpty())
		}

		return false
	})

	return newDataPoints
}
