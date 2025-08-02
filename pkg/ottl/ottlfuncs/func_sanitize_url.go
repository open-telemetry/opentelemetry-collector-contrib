// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context" // #nosec
	"errors"
	"regexp"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SanitizeURLArguments[K any] struct {
	Target ottl.StringGetter[K]
}

func NewSanitizeURLFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("SanitizeURL", &SanitizeURLArguments[K]{}, createSanitizeURLFunction[K])
}

func createSanitizeURLFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SanitizeURLArguments[K])

	if !ok {
		return nil, errors.New("SanitizeURLFactory args must be of type *SanitizeURLArguments[K]")
	}

	return SanitizeURL(args.Target)
}

var patterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	// e.g. "/api/user/12345"
	{regexp.MustCompile(`^\d+$`), "{int}"},
	// e.g. "/api/item/507f1f77bcf86cd799439011"
	{regexp.MustCompile(`^[0-9a-fA-F]{24}$`), "{objectId}"},
	// e.g. "/api/session/550e8400-e29b-41d4-a716-446655440000"
	{regexp.MustCompile(`^[0-9a-fA-F\-]{36}$`), "{uuid}"},
	// e.g. "/files/download/XyZABcDeFg1234"
	{regexp.MustCompile(`^[0-9a-zA-Z-_]{10,}$`), "{token}"},
}

func normalizeURL(url string) string {
	parts := strings.Split(url, "/")
	for i, part := range parts {
		for _, p := range patterns {
			if p.re.MatchString(part) {
				parts[i] = p.repl
				break
			}
		}
	}
	return strings.Join(parts, "/")
}

func SanitizeURL[K any](target ottl.StringGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return normalizeURL(val), nil
	}, nil
}
