package logs

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type UnrollArguments struct {
	Field ottl.GetSetter[ottllog.TransformContext]
}

// validate that they're not trying to unroll a scope attribute/resource attribute
// restrict to logRecord fields

func newUnrollFactory() ottl.Factory[ottllog.TransformContext] {
	return ottl.NewFactory("unroll", &UnrollArguments{}, createUnrollFunction)
}

func createUnrollFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottllog.TransformContext], error) {
	args, ok := oArgs.(*UnrollArguments)
	if !ok {
		return nil, fmt.Errorf("UnrollFactory args must be of type *UnrollArguments")
	}
	return unroll(args.Field)
}

func unroll(field ottl.GetSetter[ottllog.TransformContext]) (ottl.ExprFunc[ottllog.TransformContext], error) {

	var currentExpansions []pcommon.Value
	var unrollType pcommon.ValueType
	return func(ctx context.Context, tCtx ottllog.TransformContext) (any, error) {
		value, err := field.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get value to unroll: %w", err)
		}

		currentLogRecord := tCtx.GetLogRecord()
		if unrollIdx, ok := currentLogRecord.Attributes().Get("unrolled_idx"); ok {
			// we're in the middle of unrolling
			currentLogRecord.Attributes().Remove("unrolled_idx")
			switch unrollType {
			case pcommon.ValueTypeStr:
				value := currentExpansions[unrollIdx.Int()]
				currentLogRecord.Body().SetStr(value.AsString())
			case pcommon.ValueTypeInt:
				value := currentExpansions[unrollIdx.Int()]
				currentLogRecord.Body().SetInt(value.Int())
			case pcommon.ValueTypeDouble:
				value := currentExpansions[unrollIdx.Int()]
				currentLogRecord.Body().SetDouble(value.Double())
			case pcommon.ValueTypeBool:
				value := currentExpansions[unrollIdx.Int()]
				currentLogRecord.Body().SetBool(value.Bool())
			case pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
			default:
				return nil, fmt.Errorf("unable to continue unrolling %v", unrollType)
			}
			return nil, nil
		}

		expansions := []pcommon.Value{}
		switch value := value.(type) {
		case pcommon.Slice:
			for i := 0; i < value.Len(); i++ {
				v := value.At(i)
				unrollType = v.Type()
				expansions = append(expansions, v)
			}
		default:
			return nil, fmt.Errorf("input field is not a slice, got %T", value)
		}

		scopeLogs := tCtx.GetScopeLogs()
		currentRecord := tCtx.GetLogRecord()
		scopeLogs.LogRecords().RemoveIf(func(removeCandidate plog.LogRecord) bool {
			return removeCandidate == currentRecord
		})

		for idx, expansion := range expansions {
			newRecord := scopeLogs.LogRecords().AppendEmpty()
			currentRecord.CopyTo(newRecord)

			// handle current element as base
			if idx == 0 {
				switch unrollType {
				case pcommon.ValueTypeStr:
					newRecord.Body().SetStr(expansion.AsString())
				case pcommon.ValueTypeInt:
					newRecord.Body().SetInt(expansion.Int())
				case pcommon.ValueTypeDouble:
					newRecord.Body().SetDouble(expansion.Double())
				case pcommon.ValueTypeBool:
					newRecord.Body().SetBool(expansion.Bool())
				default:
					return nil, fmt.Errorf("unable to unroll %v", unrollType)
				}
				continue
			}
			// currentLength := scopeLogs.LogRecords().Len()
			newRecord.Attributes().PutInt("unrolled_idx", int64(len(currentExpansions)+idx))
			switch unrollType {
			case pcommon.ValueTypeStr:
				newRecord.Body().SetStr(expansion.AsString())
			case pcommon.ValueTypeInt:
				newRecord.Body().SetInt(expansion.Int())
			case pcommon.ValueTypeDouble:
				newRecord.Body().SetDouble(expansion.Double())
			case pcommon.ValueTypeBool:
				newRecord.Body().SetBool(expansion.Bool())
			default:
				return nil, fmt.Errorf("unable to unroll %v", unrollType)
			}
		}
		currentExpansions = append(currentExpansions, expansions...)

		return nil, nil
	}, nil
}
