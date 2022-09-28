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

// nolint:gocritic
package ottldatapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"
)

var _ ottlcommon.ResourceContext = TransformContext{}
var _ ottlcommon.InstrumentationScopeContext = TransformContext{}

type TransformContext struct {
	dataPoint            interface{}
	metric               pmetric.Metric
	metrics              pmetric.MetricSlice
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
}

func NewTransformContext(dataPoint interface{}, metric pmetric.Metric, metrics pmetric.MetricSlice, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource) TransformContext {
	return TransformContext{
		dataPoint:            dataPoint,
		metric:               metric,
		metrics:              metrics,
		instrumentationScope: instrumentationScope,
		resource:             resource,
	}
}

func (ctx TransformContext) GetDataPoint() interface{} {
	return ctx.dataPoint
}

func (ctx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.instrumentationScope
}

func (ctx TransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

func (ctx TransformContext) GetMetric() pmetric.Metric {
	return ctx.metric
}

func (ctx TransformContext) GetMetrics() pmetric.MetricSlice {
	return ctx.metrics
}

var symbolTable = map[ottl.EnumSymbol]ottl.Enum{
	"AGGREGATION_TEMPORALITY_UNSPECIFIED":    ottl.Enum(pmetric.MetricAggregationTemporalityUnspecified),
	"AGGREGATION_TEMPORALITY_DELTA":          ottl.Enum(pmetric.MetricAggregationTemporalityDelta),
	"AGGREGATION_TEMPORALITY_CUMULATIVE":     ottl.Enum(pmetric.MetricAggregationTemporalityCumulative),
	"FLAG_NONE":                              0,
	"FLAG_NO_RECORDED_VALUE":                 1,
	"METRIC_DATA_TYPE_NONE":                  ottl.Enum(pmetric.MetricTypeNone),
	"METRIC_DATA_TYPE_GAUGE":                 ottl.Enum(pmetric.MetricTypeGauge),
	"METRIC_DATA_TYPE_SUM":                   ottl.Enum(pmetric.MetricTypeSum),
	"METRIC_DATA_TYPE_HISTOGRAM":             ottl.Enum(pmetric.MetricTypeHistogram),
	"METRIC_DATA_TYPE_EXPONENTIAL_HISTOGRAM": ottl.Enum(pmetric.MetricTypeExponentialHistogram),
	"METRIC_DATA_TYPE_SUMMARY":               ottl.Enum(pmetric.MetricTypeSummary),
}

func ParseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		if enum, ok := symbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

func ParsePath(val *ottl.Path) (ottl.GetSetter[TransformContext], error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []ottl.Field) (ottl.GetSetter[TransformContext], error) {
	switch path[0].Name {
	case "resource":
		return ottlcommon.ResourcePathGetSetter[TransformContext](path[1:])
	case "instrumentation_scope":
		return ottlcommon.ScopePathGetSetter[TransformContext](path[1:])
	case "metric":
		if len(path) == 1 {
			return accessMetric(), nil
		}
		switch path[1].Name {
		case "name":
			return accessMetricName(), nil
		case "description":
			return accessMetricDescription(), nil
		case "unit":
			return accessMetricUnit(), nil
		case "type":
			return accessMetricType(), nil
		case "aggregation_temporality":
			return accessMetricAggTemporality(), nil
		case "is_monotonic":
			return accessMetricIsMonotonic(), nil
		}
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessAttributes(), nil
		}
		return accessAttributesKey(mapKey), nil
	case "start_time_unix_nano":
		return accessStartTimeUnixNano(), nil
	case "time_unix_nano":
		return accessTimeUnixNano(), nil
	case "value_double":
		return accessDoubleValue(), nil
	case "value_int":
		return accessIntValue(), nil
	case "exemplars":
		return accessExemplars(), nil
	case "flags":
		return accessFlags(), nil
	case "count":
		return accessCount(), nil
	case "sum":
		return accessSum(), nil
	case "bucket_counts":
		return accessBucketCounts(), nil
	case "explicit_bounds":
		return accessExplicitBounds(), nil
	case "scale":
		return accessScale(), nil
	case "zero_count":
		return accessZeroCount(), nil
	case "positive":
		if len(path) == 1 {
			return accessPositive(), nil
		}
		switch path[1].Name {
		case "offset":
			return accessPositiveOffset(), nil
		case "bucket_counts":
			return accessPositiveBucketCounts(), nil
		}
	case "negative":
		if len(path) == 1 {
			return accessNegative(), nil
		}
		switch path[1].Name {
		case "offset":
			return accessNegativeOffset(), nil
		case "bucket_counts":
			return accessNegativeBucketCounts(), nil
		}
	case "quantile_values":
		return accessQuantileValues(), nil
	}
	return nil, fmt.Errorf("invalid path expression %v", path)
}

func accessMetric() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ctx.GetMetric()
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newMetric, ok := val.(pmetric.Metric); ok {
				newMetric.CopyTo(ctx.GetMetric())
			}
		},
	}
}

func accessMetricName() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ctx.GetMetric().Name()
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetMetric().SetName(str)
			}
		},
	}
}

func accessMetricDescription() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ctx.GetMetric().Description()
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetMetric().SetDescription(str)
			}
		},
	}
}

func accessMetricUnit() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ctx.GetMetric().Unit()
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetMetric().SetUnit(str)
			}
		},
	}
}

func accessMetricType() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return int64(ctx.GetMetric().Type())
		},
		Setter: func(ctx TransformContext, val interface{}) {
			// TODO Implement methods so correctly convert data types.
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10130
		},
	}
}

func accessMetricAggTemporality() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			metric := ctx.GetMetric()
			switch metric.Type() {
			case pmetric.MetricTypeSum:
				return int64(metric.Sum().AggregationTemporality())
			case pmetric.MetricTypeHistogram:
				return int64(metric.Histogram().AggregationTemporality())
			case pmetric.MetricTypeExponentialHistogram:
				return int64(metric.ExponentialHistogram().AggregationTemporality())
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newAggTemporality, ok := val.(int64); ok {
				metric := ctx.GetMetric()
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				case pmetric.MetricTypeHistogram:
					metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				case pmetric.MetricTypeExponentialHistogram:
					metric.ExponentialHistogram().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				}
			}
		},
	}
}

func accessMetricIsMonotonic() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			metric := ctx.GetMetric()
			switch metric.Type() {
			case pmetric.MetricTypeSum:
				return metric.Sum().IsMonotonic()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newIsMonotonic, ok := val.(bool); ok {
				metric := ctx.GetMetric()
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					metric.Sum().SetIsMonotonic(newIsMonotonic)
				}
			}
		},
	}
}

func accessAttributes() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetDataPoint().(pmetric.NumberDataPoint).Attributes()
			case pmetric.HistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.HistogramDataPoint).Attributes()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Attributes()
			case pmetric.SummaryDataPoint:
				return ctx.GetDataPoint().(pmetric.SummaryDataPoint).Attributes()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetDataPoint().(pmetric.NumberDataPoint).Attributes())
				}
			case pmetric.HistogramDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetDataPoint().(pmetric.HistogramDataPoint).Attributes())
				}
			case pmetric.ExponentialHistogramDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Attributes())
				}
			case pmetric.SummaryDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					attrs.CopyTo(ctx.GetDataPoint().(pmetric.SummaryDataPoint).Attributes())
				}
			}
		},
	}
}

func accessAttributesKey(mapKey *string) ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return ottlcommon.GetMapValue(ctx.GetDataPoint().(pmetric.NumberDataPoint).Attributes(), *mapKey)
			case pmetric.HistogramDataPoint:
				return ottlcommon.GetMapValue(ctx.GetDataPoint().(pmetric.HistogramDataPoint).Attributes(), *mapKey)
			case pmetric.ExponentialHistogramDataPoint:
				return ottlcommon.GetMapValue(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Attributes(), *mapKey)
			case pmetric.SummaryDataPoint:
				return ottlcommon.GetMapValue(ctx.GetDataPoint().(pmetric.SummaryDataPoint).Attributes(), *mapKey)
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				ottlcommon.SetMapValue(ctx.GetDataPoint().(pmetric.NumberDataPoint).Attributes(), *mapKey, val)
			case pmetric.HistogramDataPoint:
				ottlcommon.SetMapValue(ctx.GetDataPoint().(pmetric.HistogramDataPoint).Attributes(), *mapKey, val)
			case pmetric.ExponentialHistogramDataPoint:
				ottlcommon.SetMapValue(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Attributes(), *mapKey, val)
			case pmetric.SummaryDataPoint:
				ottlcommon.SetMapValue(ctx.GetDataPoint().(pmetric.SummaryDataPoint).Attributes(), *mapKey, val)
			}
		},
	}
}

func accessStartTimeUnixNano() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetDataPoint().(pmetric.NumberDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.HistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.HistogramDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.SummaryDataPoint:
				return ctx.GetDataPoint().(pmetric.SummaryDataPoint).StartTimestamp().AsTime().UnixNano()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newTime, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetDataPoint().(pmetric.NumberDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.HistogramDataPoint:
					ctx.GetDataPoint().(pmetric.HistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.SummaryDataPoint:
					ctx.GetDataPoint().(pmetric.SummaryDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				}
			}
		},
	}
}

func accessTimeUnixNano() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetDataPoint().(pmetric.NumberDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.HistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.HistogramDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.SummaryDataPoint:
				return ctx.GetDataPoint().(pmetric.SummaryDataPoint).Timestamp().AsTime().UnixNano()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newTime, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetDataPoint().(pmetric.NumberDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.HistogramDataPoint:
					ctx.GetDataPoint().(pmetric.HistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.SummaryDataPoint:
					ctx.GetDataPoint().(pmetric.SummaryDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				}
			}
		},
	}
}

func accessDoubleValue() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetDataPoint().(pmetric.NumberDataPoint).DoubleValue()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newDouble, ok := val.(float64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetDataPoint().(pmetric.NumberDataPoint).SetDoubleValue(newDouble)
				}
			}
		},
	}
}

func accessIntValue() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetDataPoint().(pmetric.NumberDataPoint).IntValue()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newInt, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetDataPoint().(pmetric.NumberDataPoint).SetIntValue(newInt)
				}
			}
		},
	}
}

func accessExemplars() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetDataPoint().(pmetric.NumberDataPoint).Exemplars()
			case pmetric.HistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.HistogramDataPoint).Exemplars()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Exemplars()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newExemplars, ok := val.(pmetric.ExemplarSlice); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.NumberDataPoint:
					newExemplars.CopyTo(ctx.GetDataPoint().(pmetric.NumberDataPoint).Exemplars())
				case pmetric.HistogramDataPoint:
					newExemplars.CopyTo(ctx.GetDataPoint().(pmetric.HistogramDataPoint).Exemplars())
				case pmetric.ExponentialHistogramDataPoint:
					newExemplars.CopyTo(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Exemplars())
				}
			}
		},
	}
}

func accessFlags() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.NumberDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.NumberDataPoint).Flags())
			case pmetric.HistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.HistogramDataPoint).Flags())
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Flags())
			case pmetric.SummaryDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.SummaryDataPoint).Flags())
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newFlags, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetDataPoint().(pmetric.NumberDataPoint).SetFlags(pmetric.MetricDataPointFlags(newFlags))
				case pmetric.HistogramDataPoint:
					ctx.GetDataPoint().(pmetric.HistogramDataPoint).SetFlags(pmetric.MetricDataPointFlags(newFlags))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).SetFlags(pmetric.MetricDataPointFlags(newFlags))
				case pmetric.SummaryDataPoint:
					ctx.GetDataPoint().(pmetric.SummaryDataPoint).SetFlags(pmetric.MetricDataPointFlags(newFlags))
				}
			}
		},
	}
}

func accessCount() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.HistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.HistogramDataPoint).Count())
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Count())
			case pmetric.SummaryDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.SummaryDataPoint).Count())
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newCount, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetDataPoint().(pmetric.HistogramDataPoint).SetCount(uint64(newCount))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).SetCount(uint64(newCount))
				case pmetric.SummaryDataPoint:
					ctx.GetDataPoint().(pmetric.SummaryDataPoint).SetCount(uint64(newCount))
				}
			}
		},
	}
}

func accessSum() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.HistogramDataPoint).Sum()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Sum()
			case pmetric.SummaryDataPoint:
				return ctx.GetDataPoint().(pmetric.SummaryDataPoint).Sum()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newSum, ok := val.(float64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetDataPoint().(pmetric.HistogramDataPoint).SetSum(newSum)
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).SetSum(newSum)
				case pmetric.SummaryDataPoint:
					ctx.GetDataPoint().(pmetric.SummaryDataPoint).SetSum(newSum)
				}
			}
		},
	}
}

func accessExplicitBounds() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.HistogramDataPoint).ExplicitBounds().AsRaw()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newExplicitBounds, ok := val.([]float64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetDataPoint().(pmetric.HistogramDataPoint).ExplicitBounds().FromRaw(newExplicitBounds)
				}
			}
		},
	}
}

func accessBucketCounts() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.HistogramDataPoint).BucketCounts().AsRaw()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newBucketCount, ok := val.([]uint64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetDataPoint().(pmetric.HistogramDataPoint).BucketCounts().FromRaw(newBucketCount)
				}
			}
		},
	}
}

func accessScale() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Scale())
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newScale, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).SetScale(int32(newScale))
				}
			}
		},
	}
}

func accessZeroCount() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).ZeroCount())
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newZeroCount, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).SetZeroCount(uint64(newZeroCount))
				}
			}
		},
	}
}

func accessPositive() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Positive()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newPositive, ok := val.(pmetric.Buckets); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					newPositive.CopyTo(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Positive())
				}
			}
		},
	}
}

func accessPositiveOffset() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Positive().Offset())
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newPositiveOffset, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Positive().SetOffset(int32(newPositiveOffset))
				}
			}
		},
	}
}

func accessPositiveBucketCounts() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Positive().BucketCounts().AsRaw()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newPositiveBucketCounts, ok := val.([]uint64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Positive().BucketCounts().FromRaw(newPositiveBucketCounts)
				}
			}
		},
	}
}

func accessNegative() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Negative()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newNegative, ok := val.(pmetric.Buckets); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					newNegative.CopyTo(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Negative())
				}
			}
		},
	}
}

func accessNegativeOffset() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Negative().Offset())
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newNegativeOffset, ok := val.(int64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Negative().SetOffset(int32(newNegativeOffset))
				}
			}
		},
	}
}

func accessNegativeBucketCounts() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Negative().BucketCounts().AsRaw()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newNegativeBucketCounts, ok := val.([]uint64); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetDataPoint().(pmetric.ExponentialHistogramDataPoint).Negative().BucketCounts().FromRaw(newNegativeBucketCounts)
				}
			}
		},
	}
}

func accessQuantileValues() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			switch ctx.GetDataPoint().(type) {
			case pmetric.SummaryDataPoint:
				return ctx.GetDataPoint().(pmetric.SummaryDataPoint).QuantileValues()
			}
			return nil
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newQuantileValues, ok := val.(pmetric.ValueAtQuantileSlice); ok {
				switch ctx.GetDataPoint().(type) {
				case pmetric.SummaryDataPoint:
					newQuantileValues.CopyTo(ctx.GetDataPoint().(pmetric.SummaryDataPoint).QuantileValues())
				}
			}
		},
	}
}
