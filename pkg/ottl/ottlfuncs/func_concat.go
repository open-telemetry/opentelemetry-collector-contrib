// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ConcatArguments[K any] struct {
	Vals      ottl.PSliceGetter[K]
	Delimiter ottl.StringGetter[K]
}

func NewConcatFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Concat", &ConcatArguments[K]{}, createConcatFunction[K])
}

func createConcatFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ConcatArguments[K])

	if !ok {
		return nil, errors.New("ConcatFactory args must be of type *ConcatArguments[K]")
	}

	return concat(args.Vals, args.Delimiter), nil
}

func concat[K any](vals ottl.PSliceGetter[K], delimiter ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		builder := strings.Builder{}
		delimiterVal, err := delimiter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		sliceVals, err := vals.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		wroteValue := false
		for i := 0; i < sliceVals.Len(); i++ {
			val := sliceVals.At(i)
			valString := concatValueString(val)

			if wroteValue {
				builder.WriteString(delimiterVal)
			}
			builder.WriteString(valString)
			wroteValue = true
		}
		return builder.String(), nil
	}
}

func concatValueString(v pcommon.Value) string {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(v.Int(), 10)
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(v.Double(), 'g', -1, 64)
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(v.Bool())
	case pcommon.ValueTypeBytes:
		return hex.EncodeToString(v.Bytes().AsRaw())
	case pcommon.ValueTypeEmpty:
		return "<nil>"
	case pcommon.ValueTypeMap, pcommon.ValueTypeSlice:
		return v.AsString()
	default:
		return v.AsString()
	}
}
