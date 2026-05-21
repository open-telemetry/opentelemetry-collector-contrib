// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SubstringArguments[K any] struct {
	Target   ottl.StringGetter[K]
	Start    ottl.IntGetter[K]
	Length   ottl.IntGetter[K]
	Utf8Safe ottl.Optional[bool]
}

func NewSubstringFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Substring", &SubstringArguments[K]{}, createSubstringFunction[K])
}

func createSubstringFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SubstringArguments[K])

	if !ok {
		return nil, errors.New("SubstringFactory args must be of type *SubstringArguments[K]")
	}

	return substring(args.Target, args.Start, args.Length, args.Utf8Safe), nil
}

func substring[K any](
	target ottl.StringGetter[K],
	startGetter, lengthGetter ottl.IntGetter[K],
	utf8Safe ottl.Optional[bool],
) ottl.ExprFunc[K] {
	useUTF8Safe := utf8Safe.GetOr(false)
	return func(ctx context.Context, tCtx K) (any, error) {
		start, err := startGetter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if start < 0 {
			return nil, fmt.Errorf("invalid start for substring function, %d cannot be negative", start)
		}
		length, err := lengthGetter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if length <= 0 {
			return nil, fmt.Errorf("invalid length for substring function, %d cannot be negative or zero", length)
		}
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		if useUTF8Safe {
			out, err := substringRunes(val, start, length)
			if err != nil {
				return nil, err
			}
			return out, nil
		}
		if start > int64(len(val)) || length > int64(len(val))-start {
			return nil, fmt.Errorf(
				"invalid range for substring function, start(%d)+length(%d) cannot be greater than the length of target string(%d)",
				start,
				length,
				len(val),
			)
		}
		return val[start : start+length], nil
	}
}

func substringRunes(val string, start, length int64) (string, error) {
	end := start + length
	// runes ≤ bytes in UTF-8; short-circuit when end exceeds byte length.
	if end > int64(len(val)) {
		return "", fmt.Errorf(
			"invalid range for substring function, start(%d)+length(%d) exceeds byte length(%d) of target string",
			start,
			length,
			len(val),
		)
	}
	// ASCII fast path; on miss the fallback re-reads bytes still in L1.
	if isASCII(val[:end]) {
		return val[start:end], nil
	}

	var byteStart int
	var runes int64
	for i := range val {
		if runes == start {
			byteStart = i
		}
		if runes == end {
			return val[byteStart:i], nil
		}
		runes++
	}
	if runes == end {
		return val[byteStart:], nil
	}
	return "", fmt.Errorf(
		"invalid range for substring function, start(%d)+length(%d) cannot be greater than the rune length of target string(%d)",
		start,
		length,
		runes,
	)
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
