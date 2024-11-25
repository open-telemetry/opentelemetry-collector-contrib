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
	changeCounter := 0
	var unrollType pcommon.ValueType
	return func(ctx context.Context, tCtx ottllog.TransformContext) (any, error) {
		value, err := field.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get value to unroll: %w", err)
		}

		if changeCounter > 0 {
			changeCounter--
			currentLogRecord := tCtx.GetLogRecord()
			switch unrollType {
			case pcommon.ValueTypeStr:
				currentLogRecord.Body().SetStr(currentLogRecord.Body().AsString())
			case pcommon.ValueTypeInt:
				currentLogRecord.Body().SetInt(currentLogRecord.Body().Int())
			case pcommon.ValueTypeDouble:
				currentLogRecord.Body().SetDouble(currentLogRecord.Body().Double())
			case pcommon.ValueTypeBool:
				currentLogRecord.Body().SetBool(currentLogRecord.Body().Bool())
			case pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
				// do nothing
			default:
				return nil, fmt.Errorf("unable to continue unrolling%v", unrollType)
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
				value.RemoveIf(func(removeCandidate pcommon.Value) bool {
					return removeCandidate == v
				})
			}
		default:
			return nil, fmt.Errorf("input field is not a slice, got %T", value)
		}

		scopeLogs := tCtx.GetScopeLogs()

		currentRecord := tCtx.GetLogRecord()
		scopeLogs.LogRecords().RemoveIf(func(removeCandidate plog.LogRecord) bool {
			return removeCandidate == currentRecord
		})

		newLogs := plog.NewScopeLogs()
		scopeLogs.CopyTo(newLogs)
		records := newLogs.LogRecords()

		for _, expansion := range expansions {
			newRecord := records.AppendEmpty()
			currentRecord.CopyTo(newRecord)
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
			// figure out the field being set; not always going to be the body
			// newRecord.Body().SetStr(expansion)
			changeCounter++
		}
		newLogs.MoveTo(scopeLogs)

		return nil, nil
	}, nil
}
