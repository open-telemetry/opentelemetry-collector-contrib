package ottlfuncs

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type UnrollArguments[K any] struct {
	From ottl.GetSetter[K]
	To   ottl.GetSetter[K]
}

func NewUnrollFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("unroll", &UnrollArguments[K]{}, createUnrollFunction[K])
}

func createUnrollFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*UnrollArguments[K])

	if !ok {
		return nil, fmt.Errorf("UnrollFactory args must be of type *UnrollArguments[K]")
	}

	return unroll(args.From, args.To)
}

func cleanupMap(m pcommon.Map, key string) {
	if bodyVal, exists := m.Get("body"); exists && bodyVal.Type() == pcommon.ValueTypeMap {
		bodyMap := bodyVal.Map()
		bodyMap.Remove(key)
		// If body is empty after removal, remove it too
		if bodyMap.Len() == 0 {
			m.Remove("body")
		}
	}
}

func unroll[K any](from ottl.GetSetter[K], to ottl.GetSetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		fmt.Printf("Received tCtx type: %T\n", tCtx)
		// Get the original value
		value, err := from.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get value to unroll: %w", err)
		}

		var sliceVal []interface{}

		// Handle different types of input values
		switch v := value.(type) {
		case pcommon.Map:
			raw := v.AsRaw()
			// Look for any array in the map
			for key, val := range raw {
				if arr, ok := val.([]interface{}); ok {
					sliceVal = arr
					// Remove the field from the original map
					v.Remove(key)

					// If this is a log record, clean up both body and attributes
					if lr, ok := any(tCtx).(plog.LogRecord); ok {
						// Clean up from attributes if present
						if attrs := lr.Attributes(); attrs.Len() > 0 {
							cleanupMap(attrs, key)
						}

						// Clean up from body if present
						if lr.Body().Type() == pcommon.ValueTypeMap {
							cleanupMap(lr.Body().Map(), key)
						}
					}
					break
				}
			}
			if sliceVal == nil {
				return nil, fmt.Errorf("map does not contain an array value")
			}
		case []interface{}:
			sliceVal = v
			// Handle direct array case
			if lr, ok := any(tCtx).(plog.LogRecord); ok {
				// If the array was in the body, clear it
				if lr.Body().Type() == pcommon.ValueTypeMap {
					lr.Body().SetEmptyMap()
				}
			}
		default:
			return nil, fmt.Errorf("value is not an array or pcommon.Map, got %T", value)
		}

		results := make([]K, 0, len(sliceVal))

		// Create new records for each value
		if len(sliceVal) > 0 {
			for _, item := range sliceVal {
				var newTCtx K
				z := any(tCtx)
				switch v := z.(type) {
				case ottllog.TransformContext:
					newLog := plog.NewLogRecord()
					v.GetLogRecord().CopyTo(newLog)
					newScope := plog.NewScopeLogs()
					if scope := v.GetScopeSchemaURLItem(); scope != nil {
						newScope.SetSchemaUrl(scope.SchemaUrl())
					}
					newResource := plog.NewResourceLogs()
					if resource := v.GetResourceSchemaURLItem(); resource != nil {
						newResource.SetSchemaUrl(resource.SchemaUrl())
					}

					newTCtx = any(ottllog.NewTransformContext(
						newLog,
						v.GetInstrumentationScope(),
						v.GetResource(),
						newScope,
						newResource,
					)).(K)
				case plog.LogRecord:
					newLog := plog.NewLogRecord()
					v.CopyTo(newLog)
					newTCtx = any(newLog).(K)
				case ptrace.Span:
					newSpan := ptrace.NewSpan()
					v.CopyTo(newSpan)
					newTCtx = any(newSpan).(K)
				case pmetric.Metric:
					newMetric := pmetric.NewMetric()
					v.CopyTo(newMetric)
					newTCtx = any(newMetric).(K)
				default:
					return nil, fmt.Errorf("unsupported context type: %T", tCtx)
				}

				var pValue interface{} = item
				if rawValue, ok := item.(map[string]interface{}); ok {
					newMap := pcommon.NewMap()
					err := newMap.FromRaw(rawValue)
					if err != nil {
						return nil, fmt.Errorf("failed to convert map value: %w", err)
					}
					pValue = newMap
				}

				err = to.Set(ctx, newTCtx, pValue)
				if err != nil {
					return nil, fmt.Errorf("failed to set value: %w", err)
				}

				results = append(results, newTCtx)
			}
		}

		return results, nil
	}, nil
}
