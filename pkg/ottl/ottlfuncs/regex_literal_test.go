// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestLiteralRegexMatchesStandardRegexp(t *testing.T) {
	for _, pattern := range []string{
		`worker`, `worker\.name`, `(worker)`, `(?P<name>worker)`, `世界`, "\ufffd",
		`worker-\d+`, `^worker$`, `(?i)worker`, `a*`, `worker|task`, ``, `\bworker\b`,
	} {
		for _, literal := range []bool{false, true} {
			getter, err := ottl.NewTestingLiteralGetter[any, string](literal,
				ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
					return pattern, nil
				}})
			require.NoError(t, err)
			matcher, err := newDynamicRegex("test", getter)
			require.NoError(t, err)
			compiled, err := matcher.compile(t.Context(), nil)
			require.NoError(t, err)
			for _, input := range []string{"worker", "worker.name", "prefix-worker-suffix", "WORKER", "世界", "\ufffd", "\xff", "\xffworker", "", "a"} {
				require.Equal(t, regexp.MustCompile(pattern).MatchString(input),
					matcher.matchString(compiled, input), "pattern %q, input %q, literal %v", pattern, input, literal)
			}
		}
	}
}

func TestLiteralRegexDynamicPatternChanges(t *testing.T) {
	pattern := "first"
	matcher, err := newDynamicRegex("test",
		ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
			return pattern, nil
		}})
	require.NoError(t, err)
	for _, next := range []string{"first", "second", `third-\d+`} {
		pattern = next
		compiled, err := matcher.compile(t.Context(), nil)
		require.NoError(t, err)
		for _, input := range []string{"first", "second", "third-42", "missing"} {
			require.Equal(t, compiled.MatchString(input), matcher.matchString(compiled, input))
		}
	}
}

func BenchmarkReplaceAllPatternsLiteralNoMatch(b *testing.B) {
	for _, size := range []int{8, 32, 128} {
		for _, mode := range []string{modeKey, modeValue} {
			b.Run(fmt.Sprintf("%s/%d", mode, size), func(b *testing.B) {
				value := pcommon.NewMap()
				for index := range size {
					value.PutStr(fmt.Sprintf("attribute.%03d", index), "unmodified")
				}
				pattern, err := ottl.NewTestingLiteralGetter[pcommon.Map, string](true,
					ottl.StandardStringGetter[pcommon.Map]{Getter: func(context.Context, pcommon.Map) (any, error) {
						return `missing`, nil
					}})
				if err != nil {
					b.Fatal(err)
				}
				function, err := replaceAllPatterns(
					ottl.StandardPMapGetSetter[pcommon.Map]{
						Getter: func(_ context.Context, value pcommon.Map) (pcommon.Map, error) { return value, nil },
						Setter: func(context.Context, pcommon.Map, any) error { return nil },
					}, mode, pattern,
					ottl.StandardStringGetter[pcommon.Map]{Getter: func(context.Context, pcommon.Map) (any, error) {
						return "replacement", nil
					}}, ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{}, ottl.Optional[ottl.StringGetter[pcommon.Map]]{})
				if err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				for b.Loop() {
					if _, err := function(b.Context(), value); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
