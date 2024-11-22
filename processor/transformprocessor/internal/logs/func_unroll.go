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
	return func(ctx context.Context, tCtx ottllog.TransformContext) (any, error) {
		if changeCounter > 0 {
			changeCounter--
			return nil, nil
		}

		value, err := field.Get(ctx, tCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get value to unroll: %w", err)
		}

		expansions := []string{}
		switch value := value.(type) {
		case pcommon.Slice:
			for _, v := range value.AsRaw() {
				if s, ok := v.(string); ok {
					expansions = append(expansions, s)
				} else {
					return nil, fmt.Errorf("value is not a string slice, got %T", v)
				}
				value.RemoveIf(func(removeCandidate pcommon.Value) bool {
					return removeCandidate.AsRaw() == v
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
			// figure out the field being set; not always going to be the body
			newRecord.Body().SetStr(expansion)
			// TODO: This is not a safe assumption that the records processed by the transform processor are going to be in order
			changeCounter++
		}
		newLogs.MoveTo(scopeLogs)

		return nil, nil
	}, nil
}
