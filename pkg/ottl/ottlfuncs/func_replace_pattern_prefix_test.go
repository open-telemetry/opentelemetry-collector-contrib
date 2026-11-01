// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestReplacePatternPrefixMatchesRegexp(t *testing.T) {
	for _, pattern := range []string{
		`worker-\d+`, `^worker-\d+$`, `(?i)worker-\d+`, `世界\d+`, `\.`,
		`a*`, `(?:worker|task)-\d+`, `worker-(\d+)`, ``,
	} {
		for _, input := range []string{"unchanged", "worker-no-number", "worker-42 worker-73", "WORKER-42", "世界42", "a.b", "", "\xffworker-42"} {
			for _, replacement := range []string{"redacted", "$1", "$$1", ""} {
				t.Run(pattern+"/"+input+"/"+replacement, func(t *testing.T) {
					regex := regexp.MustCompile(pattern)
					want := regex.ReplaceAllString(input, replacement)
					for _, literal := range []bool{false, true} {
						value, sets := input, 0
						target := ottl.StandardGetSetter[any]{
							Getter: func(context.Context, any) (any, error) { return value, nil },
							Setter: func(_ context.Context, _ any, next any) error {
								sets++
								value = next.(string)
								return nil
							},
						}
						patternGetter, err := ottl.NewTestingLiteralGetter[any, string](literal,
							ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
								return pattern, nil
							}})
						require.NoError(t, err)
						function, err := replacePattern[any](target, patternGetter,
							ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
								return replacement, nil
							}}, ottl.Optional[ottl.FunctionGetter[any]]{}, ottl.Optional[ottl.StringGetter[any]]{})
						require.NoError(t, err)
						_, err = function(t.Context(), nil)
						require.NoError(t, err)
						require.Equal(t, want, value)
						require.Equal(t, map[bool]int{false: 0, true: 1}[regex.MatchString(input)], sets)
					}
				})
			}
		}
	}
}

func TestReplacePatternPrefixPreservesReplacementError(t *testing.T) {
	expected := errors.New("replacement getter failed")
	pattern, err := ottl.NewTestingLiteralGetter[any, string](true,
		ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
			return `worker-\d+`, nil
		}})
	require.NoError(t, err)
	function, err := replacePattern[any](
		ottl.StandardGetSetter[any]{Getter: func(context.Context, any) (any, error) {
			return "no matching prefix", nil
		}}, pattern,
		ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
			return nil, expected
		}}, ottl.Optional[ottl.FunctionGetter[any]]{}, ottl.Optional[ottl.StringGetter[any]]{})
	require.NoError(t, err)
	_, err = function(t.Context(), nil)
	require.ErrorIs(t, err, expected)
}

func BenchmarkReplacePatternLiteralPrefix(b *testing.B) {
	for _, input := range []string{"no matching prefix", "worker-no-number", "worker-42"} {
		b.Run(input, func(b *testing.B) {
			pattern, err := ottl.NewTestingLiteralGetter[any, string](true,
				ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
					return `worker-\d+`, nil
				}})
			if err != nil {
				b.Fatal(err)
			}
			function, err := replacePattern[any](
				ottl.StandardGetSetter[any]{
					Getter: func(context.Context, any) (any, error) { return input, nil },
					Setter: func(context.Context, any, any) error { return nil },
				}, pattern,
				ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
					return "worker", nil
				}}, ottl.Optional[ottl.FunctionGetter[any]]{}, ottl.Optional[ottl.StringGetter[any]]{})
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := function(b.Context(), nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
