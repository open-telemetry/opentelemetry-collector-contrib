// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ToJSONArguments[K any] struct {
	Target ottl.Getter[K]
}

func NewToJSONFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ToJSON", &ToJSONArguments[K]{}, createToJSONFunction[K])
}

func createToJSONFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ToJSONArguments[K])

	if !ok {
		return nil, errors.New("ToJSONFactory args must be of type *ToJSONArguments[K]")
	}

	return toJSON(args.Target), nil
}

// toJSON converts a value to its JSON string representation.
// It is the inverse of ParseJSON: while ParseJSON converts a JSON string into
// a pcommon.Map or pcommon.Slice, ToJSON serializes any value back into a
// compact JSON string.
//
// Supported input types:
//
//	pcommon.Map    -> JSON object string
//	pcommon.Slice  -> JSON array string
//	pcommon.Value  -> JSON value string (delegates based on underlying type)
//	map[string]any -> JSON object string
//	[]any          -> JSON array string
//	primitives     -> JSON primitive string (string, number, bool)
//
// If the input is nil (e.g. a missing map key), ToJSON returns nil, nil,
// consistent with other OTTL converters such as String and Int.
// An explicit pcommon.ValueTypeEmpty serializes as the JSON string "null".
// pcommon.ValueTypeBytes values are serialized as base64-encoded JSON strings.
func toJSON[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if val == nil {
			return nil, nil
		}

		// Convert pdata types to raw Go types before marshalling.
		raw, err := toRaw(val)
		if err != nil {
			return nil, fmt.Errorf("unsupported type for ToJSON: %w", err)
		}

		jsonBytes, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal %T to JSON: %w", raw, err)
		}

		return string(jsonBytes), nil
	}
}

// toRaw converts pdata types to raw Go types suitable for JSON marshalling.
// It recurses into []any and map[string]any so that pdata types nested inside
// list/map literals (e.g. ToJSON([attributes["m"], "x"])) are converted too,
// rather than only the top-level value.
// Non-pdata types are returned as-is.
func toRaw(val any) (any, error) {
	switch v := val.(type) {
	case pcommon.Map:
		return v.AsRaw(), nil
	case pcommon.Slice:
		return v.AsRaw(), nil
	case pcommon.Value:
		return convertValue(v)
	case []any:
		result := make([]any, len(v))
		for i, elem := range v {
			raw, err := toRaw(elem)
			if err != nil {
				return nil, err
			}
			result[i] = raw
		}
		return result, nil
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, elem := range v {
			raw, err := toRaw(elem)
			if err != nil {
				return nil, err
			}
			result[k] = raw
		}
		return result, nil
	default:
		return val, nil
	}
}

// convertValue converts a pcommon.Value to a raw Go type.
func convertValue(v pcommon.Value) (any, error) {
	switch v.Type() {
	case pcommon.ValueTypeMap:
		return v.Map().AsRaw(), nil
	case pcommon.ValueTypeSlice:
		return v.Slice().AsRaw(), nil
	case pcommon.ValueTypeStr:
		return v.Str(), nil
	case pcommon.ValueTypeBool:
		return v.Bool(), nil
	case pcommon.ValueTypeDouble:
		return v.Double(), nil
	case pcommon.ValueTypeInt:
		return v.Int(), nil
	case pcommon.ValueTypeBytes:
		return v.Bytes().AsRaw(), nil
	case pcommon.ValueTypeEmpty:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported pcommon.Value type: %v", v.Type())
	}
}
