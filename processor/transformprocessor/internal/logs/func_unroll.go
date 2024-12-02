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

type valueQueue struct {
	data []pcommon.Value
}

func (v *valueQueue) push(val pcommon.Value) {
	v.data = append(v.data, val)
}

func (v *valueQueue) pop() (pcommon.Value, error) {
	if len(v.data) == 0 {
		return pcommon.NewValueInt(-1), fmt.Errorf("no values in queue")
	}
	val := v.data[0]
	v.data = v.data[1:]
	return val, nil
}

func unroll(field ottl.GetSetter[ottllog.TransformContext]) (ottl.ExprFunc[ottllog.TransformContext], error) {
	valueQueue := valueQueue{
		data: []pcommon.Value{},
	}

	var currentExpansions []pcommon.Value
	return func(ctx context.Context, tCtx ottllog.TransformContext) (any, error) {
		value, err := field.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get value to unroll: %w", err)
		}

		currentLogRecord := tCtx.GetLogRecord()

		// Note this is a hack to store metadata on the log record
		if _, ok := currentLogRecord.Attributes().Get("is_expanded"); ok {
			value, err := valueQueue.pop()
			if err != nil {
				return nil, fmt.Errorf("unable to get value from channel")
			}
			currentLogRecord.Body().SetStr(value.AsString())
			currentLogRecord.Attributes().Remove("is_expanded")
			return nil, nil
		}

		newValues := []pcommon.Value{}
		switch value := value.(type) {
		case pcommon.Slice:
			for i := 0; i < value.Len(); i++ {
				v := value.At(i)
				newValues = append(newValues, v)
			}
		default:
			return nil, fmt.Errorf("input field is not a slice, got %T", value)
		}

		scopeLogs := tCtx.GetScopeLogs()
		scopeLogs.LogRecords().RemoveIf(func(removeCandidate plog.LogRecord) bool {
			return removeCandidate == currentLogRecord
		})

		for idx, expansion := range newValues {
			newRecord := scopeLogs.LogRecords().AppendEmpty()
			currentLogRecord.CopyTo(newRecord)
			// handle current element as base
			if idx == 0 {
				newRecord.Body().SetStr(expansion.AsString())
				continue
			}

			newRecord.Attributes().PutBool("is_expanded", true)
			valueQueue.push(expansion)
		}
		currentExpansions = append(currentExpansions, newValues...)

		return nil, nil
	}, nil
}
