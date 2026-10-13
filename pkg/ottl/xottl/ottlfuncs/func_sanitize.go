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

const defaultSanitizeReplacement = "�"

// sanitizer repairs a string so that it is valid in a given format.
type sanitizer interface {
	// Valid reports whether s already conforms to the format.
	Valid(s string) bool
	// Sanitize returns s with every invalid sequence replaced by repl.
	Sanitize(s, repl string) string
}

type utf8Sanitizer struct{}

func (utf8Sanitizer) Valid(s string) bool {
	return utf8.ValidString(s)
}

func (utf8Sanitizer) Sanitize(s, repl string) string {
	return strings.ToValidUTF8(s, repl)
}

type sanitizeArguments[K any] struct {
	Target      ottl.GetSetter[K]
	Format      ottl.StringGetter[K]
	Replacement ottl.Optional[string]
}

// NewSanitizeFactory returns a factory for the sanitize OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/xottl/ottlfuncs/README.md#sanitize
func NewSanitizeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("sanitize", &sanitizeArguments[K]{}, createSanitizeFunction[K], ottl.WithExperimental[K]())
}

func createSanitizeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*sanitizeArguments[K])
	if !ok {
		return nil, errors.New("SanitizeFactory args must be of type *sanitizeArguments[K]")
	}

	return sanitize(args.Target, args.Format, args.Replacement)
}

func sanitize[K any](target ottl.GetSetter[K], format ottl.StringGetter[K], replacement ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	repl := replacement.GetOr(defaultSanitizeReplacement)
	if !utf8.ValidString(repl) {
		return nil, fmt.Errorf("invalid replacement for sanitize function, %q is not valid UTF-8", repl)
	}

	sanGet, err := newSanitizerGetter(format)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		san, err := sanGet.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		switch v := val.(type) {
		case nil, bool, int64, float64, []byte:
			// Nothing to sanitize.
			return nil, nil
		case string:
			if san.Valid(v) {
				return nil, nil
			}
			return nil, target.Set(ctx, tCtx, san.Sanitize(v, repl))
		case pcommon.Value:
			sanitizeValue(v, san, repl)
		case pcommon.Map:
			sanitizeMap(v, san, repl)
		case pcommon.Slice:
			sanitizeSlice(v, san, repl)
		default:
			return nil, fmt.Errorf("sanitize target must be a string, map or slice, got %T", val)
		}
		return nil, target.Set(ctx, tCtx, val)
	}, nil
}

type sanitizerGetter[K any] struct {
	sanitizer sanitizer
	getter    ottl.StringGetter[K]
}

func newSanitizerGetter[K any](getter ottl.StringGetter[K]) (*sanitizerGetter[K], error) {
	if format, isLiteral := ottl.GetLiteralValue(getter); isLiteral {
		san, err := asSanitizer(format)
		if err != nil {
			return nil, err
		}
		return &sanitizerGetter[K]{sanitizer: san}, nil
	}
	return &sanitizerGetter[K]{getter: getter}, nil
}

func (s *sanitizerGetter[K]) Get(ctx context.Context, tCtx K) (sanitizer, error) {
	if s.sanitizer != nil {
		return s.sanitizer, nil
	}
	format, err := s.getter.Get(ctx, tCtx)
	if err != nil {
		return nil, err
	}
	return asSanitizer(format)
}

func asSanitizer(format string) (sanitizer, error) {
	switch strings.ToLower(format) {
	case "utf-8", "utf8":
		return utf8Sanitizer{}, nil
	default:
		return nil, fmt.Errorf("unsupported format for sanitize function: %q, supported formats are \"utf-8\"", format)
	}
}

func sanitizeValue(v pcommon.Value, san sanitizer, repl string) {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		if s := v.Str(); !san.Valid(s) {
			v.SetStr(san.Sanitize(s, repl))
		}
	case pcommon.ValueTypeMap:
		sanitizeMap(v.Map(), san, repl)
	case pcommon.ValueTypeSlice:
		sanitizeSlice(v.Slice(), san, repl)
	}
}

func sanitizeSlice(s pcommon.Slice, san sanitizer, repl string) {
	for _, v := range s.All() {
		sanitizeValue(v, san, repl)
	}
}

func sanitizeMap(m pcommon.Map, san sanitizer, repl string) {
	var invalidKeys []string
	for k, v := range m.All() {
		sanitizeValue(v, san, repl)
		if !san.Valid(k) {
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
		moved.CopyTo(m.PutEmpty(san.Sanitize(k, repl)))
	}
}
