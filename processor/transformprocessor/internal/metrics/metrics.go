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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type metricTransformContext struct {
	metric   pmetric.Metric
	il       pcommon.InstrumentationScope
	resource pcommon.Resource
}

func (ctx metricTransformContext) GetItem() interface{} {
	return ctx.metric
}

func (ctx metricTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.il
}

func (ctx metricTransformContext) GetResource() pcommon.Resource {
	return ctx.resource
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
	return newPathGetSetter(val.Fields)
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
	case "descriptor":
		if len(path) == 1 {
			return accessDescriptor(), nil
		}
		switch path[1].Name {
		case "metric_name":
			return accessDescriptorMetricName(), nil
		case "metric_description":
			return accessDescriptorMetricDescription(), nil
		case "metric_unit":
			return accessDescriptorMetricUnit(), nil
		case "metric_type":
			return accessDescriptorMetricType(), nil
		}
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

func accessDescriptor() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pmetric.Metric)
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newMetric, ok := val.(pmetric.Metric); ok {
				newMetric.CopyTo(ctx.GetItem().(pmetric.Metric))
			}
		},
	}
}

func accessDescriptorMetricName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pmetric.Metric).Name()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(pmetric.Metric).SetName(str)
			}
		},
	}
}

func accessDescriptorMetricDescription() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pmetric.Metric).Description()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(pmetric.Metric).SetDescription(str)
			}
		},
	}
}

func accessDescriptorMetricUnit() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pmetric.Metric).Unit()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(pmetric.Metric).SetUnit(str)
			}
		},
	}
}

func accessDescriptorMetricType() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pmetric.Metric).DataType()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if dataType, ok := val.(pmetric.MetricDataType); ok {
				ctx.GetItem().(pmetric.Metric).SetDataType(dataType)
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
		return val.BytesVal()
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
		attrs.UpsertBytes(mapKey, v)
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
			arr.SliceVal().AppendEmpty().SetBytesVal(b)
		}
		attrs.Upsert(mapKey, arr)
	default:
		// TODO(anuraaga): Support set of map type.
	}
}

func setValue(value pcommon.Value, val interface{}) {
	switch v := val.(type) {
	case string:
		value.SetStringVal(v)
	case bool:
		value.SetBoolVal(v)
	case int64:
		value.SetIntVal(v)
	case float64:
		value.SetDoubleVal(v)
	case []byte:
		value.SetBytesVal(v)
	case []string:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, str := range v {
			value.SliceVal().AppendEmpty().SetStringVal(str)
		}
	case []bool:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, b := range v {
			value.SliceVal().AppendEmpty().SetBoolVal(b)
		}
	case []int64:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, i := range v {
			value.SliceVal().AppendEmpty().SetIntVal(i)
		}
	case []float64:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, f := range v {
			value.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
	case [][]byte:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, b := range v {
			value.SliceVal().AppendEmpty().SetBytesVal(b)
		}
	default:
		// TODO(anuraaga): Support set of map type.
	}
}
