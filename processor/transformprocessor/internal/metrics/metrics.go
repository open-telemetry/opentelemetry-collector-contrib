// Copyright  The OpenTelemetry Authors
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
package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type metricTransformContext struct {
	dataPoint interface{}
	metric    pmetric.Metric
	metrics   pmetric.MetricSlice
	il        pcommon.InstrumentationScope
	resource  pcommon.Resource
}

func (ctx metricTransformContext) GetItem() interface{} {
	return ctx.dataPoint
}

func (ctx metricTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.il
}

func (ctx metricTransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

func (ctx metricTransformContext) GetMetric() pmetric.Metric {
	return ctx.metric
}

func (ctx metricTransformContext) GetMetrics() pmetric.MetricSlice {
	return ctx.metrics
}

// pathGetSetter is a getSetter which has been resolved using a path expression provided by a user.
type pathGetSetter struct {
	getter common.ExprFunc
	setter func(ctx common.TransformContext, val interface{})
}

func (path pathGetSetter) Get(ctx common.TransformContext) interface{} {
	return path.getter(ctx)
}

func (path pathGetSetter) Set(ctx common.TransformContext, val interface{}) {
	path.setter(ctx, val)
}

func ParsePath(val *common.Path) (common.GetSetter, error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []common.Field) (common.GetSetter, error) {
	switch path[0].Name {
	case "resource":
		if len(path) == 1 {
			return accessResource(), nil
		}
		switch path[1].Name {
		case "attributes":
			mapKey := path[1].MapKey
			if mapKey == nil {
				return accessResourceAttributes(), nil
			}
			return accessResourceAttributesKey(mapKey), nil
		}
	case "instrumentation_scope":
		if len(path) == 1 {
			return accessInstrumentationScope(), nil
		}
		switch path[1].Name {
		case "name":
			return accessInstrumentationScopeName(), nil
		case "version":
			return accessInstrumentationScopeVersion(), nil
		}
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

func accessResource() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetResource()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newRes, ok := val.(pcommon.Resource); ok {
				ctx.GetResource().Attributes().Clear()
				newRes.CopyTo(ctx.GetResource())
			}
		},
	}
}

func accessResourceAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetResource().Attributes()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				ctx.GetResource().Attributes().Clear()
				attrs.CopyTo(ctx.GetResource().Attributes())
			}
		},
	}
}

func accessResourceAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return getAttr(ctx.GetResource().Attributes(), *mapKey)
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			setAttr(ctx.GetResource().Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationScope() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetInstrumentationScope()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newIl, ok := val.(pcommon.InstrumentationScope); ok {
				newIl.CopyTo(ctx.GetInstrumentationScope())
			}
		},
	}
}

func accessInstrumentationScopeName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Name()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetName(str)
			}
		},
	}
}

func accessInstrumentationScopeVersion() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Version()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetVersion(str)
			}
		},
	}
}

func accessMetric() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.(metricTransformContext).GetMetric()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newMetric, ok := val.(pmetric.Metric); ok {
				newMetric.CopyTo(ctx.(metricTransformContext).GetMetric())
			}
		},
	}
}

func accessMetricName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.(metricTransformContext).GetMetric().Name()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.(metricTransformContext).GetMetric().SetName(str)
			}
		},
	}
}

func accessMetricDescription() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.(metricTransformContext).GetMetric().Description()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.(metricTransformContext).GetMetric().SetDescription(str)
			}
		},
	}
}

func accessMetricUnit() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.(metricTransformContext).GetMetric().Unit()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.(metricTransformContext).GetMetric().SetUnit(str)
			}
		},
	}
}

func accessMetricType() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.(metricTransformContext).GetMetric().DataType().String()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			// TODO Implement methods so correctly convert data types.
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/10130
		},
	}
}

func accessMetricAggTemporality() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			metric := ctx.(metricTransformContext).GetMetric()
			switch metric.DataType() {
			case pmetric.MetricDataTypeSum:
				return int64(metric.Sum().AggregationTemporality())
			case pmetric.MetricDataTypeHistogram:
				return int64(metric.Histogram().AggregationTemporality())
			case pmetric.MetricDataTypeExponentialHistogram:
				return int64(metric.ExponentialHistogram().AggregationTemporality())
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newAggTemporality, ok := val.(int64); ok {
				metric := ctx.(metricTransformContext).GetMetric()
				switch metric.DataType() {
				case pmetric.MetricDataTypeSum:
					metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				case pmetric.MetricDataTypeHistogram:
					metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				case pmetric.MetricDataTypeExponentialHistogram:
					metric.ExponentialHistogram().SetAggregationTemporality(pmetric.MetricAggregationTemporality(newAggTemporality))
				}
			}
		},
	}
}

func accessMetricIsMonotonic() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			metric := ctx.(metricTransformContext).GetMetric()
			switch metric.DataType() {
			case pmetric.MetricDataTypeSum:
				return metric.Sum().IsMonotonic()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newIsMonotonic, ok := val.(bool); ok {
				metric := ctx.(metricTransformContext).GetMetric()
				switch metric.DataType() {
				case pmetric.MetricDataTypeSum:
					metric.Sum().SetIsMonotonic(newIsMonotonic)
				}
			}
		},
	}
}

func accessAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).Attributes()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Attributes()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).Attributes()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					ctx.GetItem().(pmetric.NumberDataPoint).Attributes().Clear()
					attrs.CopyTo(ctx.GetItem().(pmetric.NumberDataPoint).Attributes())
				}
			case pmetric.HistogramDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					ctx.GetItem().(pmetric.HistogramDataPoint).Attributes().Clear()
					attrs.CopyTo(ctx.GetItem().(pmetric.HistogramDataPoint).Attributes())
				}
			case pmetric.ExponentialHistogramDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes().Clear()
					attrs.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes())
				}
			case pmetric.SummaryDataPoint:
				if attrs, ok := val.(pcommon.Map); ok {
					ctx.GetItem().(pmetric.SummaryDataPoint).Attributes().Clear()
					attrs.CopyTo(ctx.GetItem().(pmetric.SummaryDataPoint).Attributes())
				}
			}
		},
	}
}

func accessAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return getAttr(ctx.GetItem().(pmetric.NumberDataPoint).Attributes(), *mapKey)
			case pmetric.HistogramDataPoint:
				return getAttr(ctx.GetItem().(pmetric.HistogramDataPoint).Attributes(), *mapKey)
			case pmetric.ExponentialHistogramDataPoint:
				return getAttr(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes(), *mapKey)
			case pmetric.SummaryDataPoint:
				return getAttr(ctx.GetItem().(pmetric.SummaryDataPoint).Attributes(), *mapKey)
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				setAttr(ctx.GetItem().(pmetric.NumberDataPoint).Attributes(), *mapKey, val)
			case pmetric.HistogramDataPoint:
				setAttr(ctx.GetItem().(pmetric.HistogramDataPoint).Attributes(), *mapKey, val)
			case pmetric.ExponentialHistogramDataPoint:
				setAttr(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Attributes(), *mapKey, val)
			case pmetric.SummaryDataPoint:
				setAttr(ctx.GetItem().(pmetric.SummaryDataPoint).Attributes(), *mapKey, val)
			}
		},
	}
}

func accessStartTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).StartTimestamp().AsTime().UnixNano()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).StartTimestamp().AsTime().UnixNano()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newTime, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				}
			}
		},
	}
}

func accessTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Timestamp().AsTime().UnixNano()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).Timestamp().AsTime().UnixNano()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newTime, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTime)))
				}
			}
		},
	}
}

func accessDoubleValue() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).DoubleVal()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newDouble, ok := val.(float64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetDoubleVal(newDouble)
				}
			}
		},
	}
}

func accessIntValue() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).IntVal()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newInt, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetIntVal(newInt)
				}
			}
		},
	}
}

func accessExemplars() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).Exemplars()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Exemplars()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Exemplars()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newExemplars, ok := val.(pmetric.ExemplarSlice); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					newExemplars.CopyTo(ctx.GetItem().(pmetric.NumberDataPoint).Exemplars())
				case pmetric.HistogramDataPoint:
					newExemplars.CopyTo(ctx.GetItem().(pmetric.HistogramDataPoint).Exemplars())
				case pmetric.ExponentialHistogramDataPoint:
					newExemplars.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Exemplars())
				}
			}
		},
	}
}

func accessFlags() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.NumberDataPoint:
				return ctx.GetItem().(pmetric.NumberDataPoint).Flags()
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Flags()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Flags()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).Flags()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newFlags, ok := val.(pmetric.MetricDataPointFlags); ok {
				switch ctx.GetItem().(type) {
				case pmetric.NumberDataPoint:
					ctx.GetItem().(pmetric.NumberDataPoint).SetFlags(newFlags)
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetFlags(newFlags)
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetFlags(newFlags)
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetFlags(newFlags)
				}
			}
		},
	}
}

func accessCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.HistogramDataPoint).Count())
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Count())
			case pmetric.SummaryDataPoint:
				return int64(ctx.GetItem().(pmetric.SummaryDataPoint).Count())
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newCount, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetCount(uint64(newCount))
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetCount(uint64(newCount))
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetCount(uint64(newCount))
				}
			}
		},
	}
}

func accessSum() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).Sum()
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Sum()
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).Sum()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newSum, ok := val.(float64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetSum(newSum)
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetSum(newSum)
				case pmetric.SummaryDataPoint:
					ctx.GetItem().(pmetric.SummaryDataPoint).SetSum(newSum)
				}
			}
		},
	}
}

func accessExplicitBounds() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).MExplicitBounds()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newExplicitBounds, ok := val.([]float64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetMExplicitBounds(newExplicitBounds)
				}
			}
		},
	}
}

func accessBucketCounts() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.HistogramDataPoint:
				return ctx.GetItem().(pmetric.HistogramDataPoint).MBucketCounts()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newBucketCount, ok := val.([]uint64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.HistogramDataPoint:
					ctx.GetItem().(pmetric.HistogramDataPoint).SetMBucketCounts(newBucketCount)
				}
			}
		},
	}
}

func accessScale() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Scale())
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newScale, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetScale(int32(newScale))
				}
			}
		},
	}
}

func accessZeroCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).ZeroCount())
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newZeroCount, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).SetZeroCount(uint64(newZeroCount))
				}
			}
		},
	}
}

func accessPositive() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newPositive, ok := val.(pmetric.Buckets); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					newPositive.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive())
				}
			}
		},
	}
}

func accessPositiveOffset() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().Offset())
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newPositiveOffset, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().SetOffset(int32(newPositiveOffset))
				}
			}
		},
	}
}

func accessPositiveBucketCounts() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().MBucketCounts()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newPositiveBucketCounts, ok := val.([]uint64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Positive().SetMBucketCounts(newPositiveBucketCounts)
				}
			}
		},
	}
}

func accessNegative() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newNegative, ok := val.(pmetric.Buckets); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					newNegative.CopyTo(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative())
				}
			}
		},
	}
}

func accessNegativeOffset() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return int64(ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().Offset())
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newNegativeOffset, ok := val.(int64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().SetOffset(int32(newNegativeOffset))
				}
			}
		},
	}
}

func accessNegativeBucketCounts() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.ExponentialHistogramDataPoint:
				return ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().MBucketCounts()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newNegativeBucketCounts, ok := val.([]uint64); ok {
				switch ctx.GetItem().(type) {
				case pmetric.ExponentialHistogramDataPoint:
					ctx.GetItem().(pmetric.ExponentialHistogramDataPoint).Negative().SetMBucketCounts(newNegativeBucketCounts)
				}
			}
		},
	}
}

func accessQuantileValues() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			switch ctx.GetItem().(type) {
			case pmetric.SummaryDataPoint:
				return ctx.GetItem().(pmetric.SummaryDataPoint).QuantileValues()
			}
			return nil
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newQuantileValues, ok := val.(pmetric.ValueAtQuantileSlice); ok {
				switch ctx.GetItem().(type) {
				case pmetric.SummaryDataPoint:
					newQuantileValues.CopyTo(ctx.GetItem().(pmetric.SummaryDataPoint).QuantileValues())
				}
			}
		},
	}
}

func getAttr(attrs pcommon.Map, mapKey string) interface{} {
	val, ok := attrs.Get(mapKey)
	if !ok {
		return nil
	}
	return getValue(val)
}

func getValue(val pcommon.Value) interface{} {
	switch val.Type() {
	case pcommon.ValueTypeString:
		return val.StringVal()
	case pcommon.ValueTypeBool:
		return val.BoolVal()
	case pcommon.ValueTypeInt:
		return val.IntVal()
	case pcommon.ValueTypeDouble:
		return val.DoubleVal()
	case pcommon.ValueTypeMap:
		return val.MapVal()
	case pcommon.ValueTypeSlice:
		return val.SliceVal()
	case pcommon.ValueTypeBytes:
		return val.MBytesVal()
	}
	return nil
}

func setAttr(attrs pcommon.Map, mapKey string, val interface{}) {
	switch v := val.(type) {
	case string:
		attrs.UpsertString(mapKey, v)
	case bool:
		attrs.UpsertBool(mapKey, v)
	case int64:
		attrs.UpsertInt(mapKey, v)
	case float64:
		attrs.UpsertDouble(mapKey, v)
	case []byte:
		attrs.UpsertBytes(mapKey, pcommon.NewImmutableByteSlice(v))
	case []string:
		arr := pcommon.NewValueSlice()
		for _, str := range v {
			arr.SliceVal().AppendEmpty().SetStringVal(str)
		}
		attrs.Upsert(mapKey, arr)
	case []bool:
		arr := pcommon.NewValueSlice()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBoolVal(b)
		}
		attrs.Upsert(mapKey, arr)
	case []int64:
		arr := pcommon.NewValueSlice()
		for _, i := range v {
			arr.SliceVal().AppendEmpty().SetIntVal(i)
		}
		attrs.Upsert(mapKey, arr)
	case []float64:
		arr := pcommon.NewValueSlice()
		for _, f := range v {
			arr.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
		attrs.Upsert(mapKey, arr)
	case [][]byte:
		arr := pcommon.NewValueSlice()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice(b))
		}
		attrs.Upsert(mapKey, arr)
	default:
		// TODO(anuraaga): Support set of map type.
	}
}
