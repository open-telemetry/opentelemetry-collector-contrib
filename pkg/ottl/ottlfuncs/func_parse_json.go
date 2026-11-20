// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type parseJSONArguments[K any] struct {
	Target    ottl.StringGetter[K]
	ParseInts ottl.Optional[bool]
}

// NewParseJSONFactory returns a factory for the ParseJSON OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/ottlfuncs/README.md#parsejson
func NewParseJSONFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseJSON", &parseJSONArguments[K]{}, createParseJSONFunction[K])
}

func createParseJSONFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*parseJSONArguments[K])

	if !ok {
		return nil, errors.New("ParseJSONFactory args must be of type *parseJSONArguments[K]")
	}

	parseInts := !args.ParseInts.IsEmpty() && args.ParseInts.Get()
	return parseJSON(args.Target, parseInts), nil
}

// parseJSON returns a `pcommon.Map` or `pcommon.Slice` struct that is a result of parsing the target string as JSON
// Each JSON type is converted into a `pdata.Value` using the following map:
//
//	JSON boolean -> bool
//	JSON number  -> float64 if parseInts is false, otherwise int64 or float64 according to the field type
//	JSON string  -> string
//	JSON null    -> nil
//	JSON arrays  -> pdata.SliceValue
//	JSON objects -> map[string]any
func parseJSON[K any](target ottl.StringGetter[K], parseInts bool) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		parsedValue, err := unmarshalJSON(ctx, tCtx, target, parseInts)
		if err != nil {
			return nil, err
		}

		switch v := parsedValue.(type) {
		case []any:
			if parseInts {
				convertNumbersArray(v)
			}
			result := pcommon.NewSlice()
			err = result.FromRaw(v)
			return result, err
		case map[string]any:
			if parseInts {
				convertNumbers(v)
			}
			result := pcommon.NewMap()
			err = result.FromRaw(v)
			return result, err
		default:
			return nil, fmt.Errorf("could not convert parsed value of type %T to JSON object", v)
		}
	}
}

func unmarshalJSON[K any](ctx context.Context, tCtx K, target ottl.StringGetter[K], parseInts bool) (any, error) {
	targetVal, err := target.Get(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	var parsedValue any

	// when parseInts is disabled, `int` and `float` will be parsed as `float64`.
	// when it is enabled, they will be parsed as `json.Number`, later the parser
	// will convert them to `int` or `float64` according to the field type.
	if !parseInts {
		err = json.Unmarshal([]byte(targetVal), &parsedValue)
		if err != nil {
			return nil, err
		}

		return parsedValue, nil
	}

	d := json.NewDecoder(strings.NewReader(targetVal))
	d.UseNumber()
	err = d.Decode(&parsedValue)
	if err != nil {
		return nil, err
	}

	// the decoder stops after the first value, so a second one needed
	// to reject data the same way `json.Unmarshal` does.
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("unexpected data after JSON value")
		}
		return nil, err
	}

	return parsedValue, nil
}

func convertNumbers(parsedValue map[string]any) {
	for k, v := range parsedValue {
		switch t := v.(type) {
		case json.Number:
			parsedValue[k] = convertNumber(t)
		case map[string]any:
			convertNumbers(t)
		case []any:
			convertNumbersArray(t)
		}
	}
}

func convertNumbersArray(arr []any) {
	for i, v := range arr {
		switch t := v.(type) {
		case json.Number:
			arr[i] = convertNumber(t)
		case map[string]any:
			convertNumbers(t)
		case []any:
			convertNumbersArray(t)
		}
	}
}

func convertNumber(value json.Number) any {
	i64, err := value.Int64()
	if err == nil {
		return i64
	}
	f64, err := value.Float64()
	if err == nil {
		return f64
	}
	return value.String()
}
