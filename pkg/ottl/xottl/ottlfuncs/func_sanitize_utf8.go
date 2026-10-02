// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const defaultUTF8Replacement = "�"

type sanitizeUTF8Arguments[K any] struct {
	Target      ottl.GetSetter[K]
	Replacement ottl.Optional[string]
}

// NewSanitizeUTF8Factory returns a factory for the sanitize_utf8 OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/xottl/ottlfuncs/README.md#sanitize_utf8
func NewSanitizeUTF8Factory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("sanitize_utf8", &sanitizeUTF8Arguments[K]{}, createSanitizeUTF8Function[K])
}

func createSanitizeUTF8Function[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*sanitizeUTF8Arguments[K])
	if !ok {
		return nil, errors.New("SanitizeUTF8Factory args must be of type *sanitizeUTF8Arguments[K]")
	}

	return sanitizeUTF8(args.Target, args.Replacement)
}

func sanitizeUTF8[K any](target ottl.GetSetter[K], replacement ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	repl := replacement.GetOr(defaultUTF8Replacement)
	if !utf8.ValidString(repl) {
		return nil, fmt.Errorf("invalid replacement for sanitize_utf8 function, %q is not valid UTF-8", repl)
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		switch v := val.(type) {
		case nil, bool, int64, float64, []byte:
			// Nothing to sanitize.
			return nil, nil
		case string:
			if utf8.ValidString(v) {
				return nil, nil
			}
			return nil, target.Set(ctx, tCtx, strings.ToValidUTF8(v, repl))
		case pcommon.Value:
			sanitizeUTF8Value(v, repl)
		case pcommon.Map:
			sanitizeUTF8Map(v, repl)
		case pcommon.Slice:
			sanitizeUTF8Slice(v, repl)
		default:
			return nil, fmt.Errorf("sanitize_utf8 target must be a string, map or slice, got %T", val)
		}
		return nil, target.Set(ctx, tCtx, val)
	}, nil
}

func sanitizeUTF8Value(v pcommon.Value, repl string) {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		if s := v.Str(); !utf8.ValidString(s) {
			v.SetStr(strings.ToValidUTF8(s, repl))
		}
	case pcommon.ValueTypeMap:
		sanitizeUTF8Map(v.Map(), repl)
	case pcommon.ValueTypeSlice:
		sanitizeUTF8Slice(v.Slice(), repl)
	}
}

func sanitizeUTF8Slice(s pcommon.Slice, repl string) {
	for _, v := range s.All() {
		sanitizeUTF8Value(v, repl)
	}
}

func sanitizeUTF8Map(m pcommon.Map, repl string) {
	var invalidKeys []string
	for k, v := range m.All() {
		sanitizeUTF8Value(v, repl)
		if !utf8.ValidString(k) {
			invalidKeys = append(invalidKeys, k)
		}
	}
	// Keys cannot be renamed in place, so entries with an invalid key are moved to the sanitized key.
	// When the sanitized key already exists its value is overwritten.
	for _, k := range invalidKeys {
		v, _ := m.Get(k)
		moved := pcommon.NewValueEmpty()
		v.CopyTo(moved)
		m.Remove(k)
		moved.CopyTo(m.PutEmpty(strings.ToValidUTF8(k, repl)))
	}
}
