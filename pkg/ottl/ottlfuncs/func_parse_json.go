// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ParseJSONArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewParseJSONFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseJSON", &ParseJSONArguments[K]{}, createParseJSONFunction[K])
}

func createParseJSONFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseJSONArguments[K])

	if !ok {
		return nil, errors.New("ParseJSONFactory args must be of type *ParseJSONArguments[K]")
	}

	return parseJSON(args.Target), nil
}

// parseJSON returns a `pcommon.Map` or `pcommon.Slice` struct that is a result of parsing the target string as JSON
// Each JSON type is converted into a `pdata.Value` using the following map:
//
//	JSON boolean -> bool
//	JSON number  -> float64
//	JSON string  -> string
//	JSON null    -> nil
//	JSON arrays  -> pdata.Slice
//	JSON objects -> pcommon.Map
func parseJSON[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		iter := jsoniter.ConfigFastest.BorrowIterator([]byte(targetVal))
		defer jsoniter.ConfigFastest.ReturnIterator(iter)
		val := pcommon.NewValueEmpty()
		unmarshalValue(val, iter)
		if iter.Error != nil {
			return nil, iter.Error
		}
		switch val.Type() {
		case pcommon.ValueTypeSlice:
			return val.Slice(), nil
		case pcommon.ValueTypeMap:
			return val.Map(), nil
		default:
			return nil, fmt.Errorf("could not convert parsed value of type %q to JSON object", val.Type())
		}
	}
}

func unmarshalValue(dest pcommon.Value, iter *jsoniter.Iterator) {
	switch iter.WhatIsNext() {
	case jsoniter.NilValue:
		// do nothing
	case jsoniter.StringValue:
		dest.SetStr(string(iter.ReadStringAsSlice()))
	case jsoniter.NumberValue:
		dest.SetDouble(iter.ReadFloat64())
	case jsoniter.BoolValue:
		dest.SetBool(iter.ReadBool())
	case jsoniter.ArrayValue:
		a := dest.SetEmptySlice()
		for iter.ReadArray() {
			unmarshalValue(a.AppendEmpty(), iter)
		}
	case jsoniter.ObjectValue:
		m := dest.SetEmptyMap()
		for f := iter.ReadObject(); f != ""; f = iter.ReadObject() {
			unmarshalValue(m.PutEmpty(f), iter)
		}
	default:
		iter.ReportError("unmarshalValue", "unknown json format")
		return
	}
}
