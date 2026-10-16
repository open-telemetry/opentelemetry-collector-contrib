// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

const defaultSanitizeReplacement = "�"

// sanitizer repairs a string so that it is valid in a given format.
type sanitizer interface {
	// Name is the format name as accepted by the function.
	Name() string
	// Valid reports whether s already conforms to the format.
	Valid(s string) bool
	// Sanitize returns s with every invalid sequence replaced by repl.
	Sanitize(s, repl string) string
}

type utf8Sanitizer struct{}

func (utf8Sanitizer) Name() string {
	return "utf-8"
}

func (utf8Sanitizer) Valid(s string) bool {
	return utf8.ValidString(s)
}

func (utf8Sanitizer) Sanitize(s, repl string) string {
	return strings.ToValidUTF8(s, repl)
}

type sanitizeArguments[K any] struct {
	Target      ottl.StringGetter[K]
	Format      ottl.StringGetter[K]
	Replacement ottl.Optional[string]
}

// NewSanitizeFactory returns a factory for the Sanitize OTTL function.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/xottl/ottlfuncs/README.md#sanitize
func NewSanitizeFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Sanitize", &sanitizeArguments[K]{}, createSanitizeFunction[K], ottl.WithExperimental[K]())
}

func createSanitizeFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*sanitizeArguments[K])
	if !ok {
		return nil, errors.New("SanitizeFactory args must be of type *sanitizeArguments[K]")
	}

	return sanitize(args.Target, args.Format, args.Replacement)
}

func sanitize[K any](target, format ottl.StringGetter[K], replacement ottl.Optional[string]) (ottl.ExprFunc[K], error) {
	repl := replacement.GetOr(defaultSanitizeReplacement)

	sanGet, err := newSanitizerGetter(format)
	if err != nil {
		return nil, err
	}
	if sanGet.sanitizer != nil {
		// Literal format: the replacement can be checked once, at parse time.
		if replErr := checkReplacement(sanGet.sanitizer, repl); replErr != nil {
			return nil, replErr
		}
	}

	return func(ctx context.Context, tCtx K) (any, error) {
		san, err := sanGet.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if replErr := checkReplacement(san, repl); replErr != nil {
			return nil, replErr
		}
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if san.Valid(val) {
			return val, nil
		}
		return san.Sanitize(val, repl), nil
	}, nil
}

func checkReplacement(san sanitizer, repl string) error {
	if !san.Valid(repl) {
		return fmt.Errorf("invalid replacement for Sanitize function, %q is not valid %s", repl, san.Name())
	}
	return nil
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
	switch format {
	case "utf-8":
		return utf8Sanitizer{}, nil
	default:
		return nil, fmt.Errorf("unsupported format for Sanitize function: %q, supported formats are \"utf-8\"", format)
	}
}
