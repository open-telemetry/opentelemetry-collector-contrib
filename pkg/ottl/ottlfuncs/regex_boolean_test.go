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

func TestIsMatchLiteralAndRegex(t *testing.T) {
	for _, pattern := range []string{
		`worker`, `worker\.name`, `世界`, "\ufffd", `^worker$`,
		`(?i)worker`, `(worker)`, `worker|task`, `worker-\d+`, ``,
	} {
		for _, literal := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/static=%v", pattern, literal), func(t *testing.T) {
				getter, err := ottl.NewTestingLiteralGetter[any, string](literal,
					ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
						return pattern, nil
					}})
				require.NoError(t, err)
				var input any
				function, err := isMatch[any](
					ottl.StandardStringLikeGetter[any]{Getter: func(context.Context, any) (any, error) {
						return input, nil
					}}, getter,
				)
				require.NoError(t, err)
				var result any
				for _, value := range []string{"worker", "prefix-worker-suffix", "worker.name", "WORKER", "worker-42", "世界", "\ufffd", "\xff", "\xffworker", "", "missing"} {
					input = value
					result, err = function(t.Context(), nil)
					require.NoError(t, err)
					require.Equal(t, regexp.MustCompile(pattern).MatchString(value), result, "input %q", value)
				}
				input = nil
				result, err = function(t.Context(), nil)
				require.NoError(t, err)
				require.Equal(t, false, result)
			})
		}
	}
}

func TestMatchingKeysLiteralAndRegex(t *testing.T) {
	for _, keep := range []bool{false, true} {
		for _, pattern := range []string{
			`worker`, `worker\.name`, `世界`, "\ufffd", `^worker$`,
			`(?i)worker`, `(worker)`, `worker|task`, `worker-\d+`, ``,
		} {
			for _, literal := range []bool{false, true} {
				t.Run(fmt.Sprintf("keep=%v/%s/static=%v", keep, pattern, literal), func(t *testing.T) {
					getter, err := ottl.NewTestingLiteralGetter[pcommon.Map, string](literal,
						ottl.StandardStringGetter[pcommon.Map]{Getter: func(context.Context, pcommon.Map) (any, error) {
							return pattern, nil
						}})
					require.NoError(t, err)
					var sets int
					target := ottl.StandardPMapGetSetter[pcommon.Map]{
						Getter: func(_ context.Context, value pcommon.Map) (pcommon.Map, error) { return value, nil },
						Setter: func(context.Context, pcommon.Map, any) error {
							sets++
							return nil
						},
					}
					factory := deleteMatchingKeys[pcommon.Map]
					if keep {
						factory = keepMatchingKeys[pcommon.Map]
					}
					function, err := factory(target, getter)
					require.NoError(t, err)
					input := pcommon.NewMap()
					expected := pcommon.NewMap()
					regex := regexp.MustCompile(pattern)
					for index, key := range []string{"worker", "prefix-worker-suffix", "worker.name", "WORKER", "worker-42", "世界", "\ufffd", "\xff", "\xffworker", "", "missing"} {
						input.PutInt(key, int64(index))
						if regex.MatchString(key) == keep {
							expected.PutInt(key, int64(index))
						}
					}
					_, err = function(t.Context(), input)
					require.NoError(t, err)
					require.Equal(t, expected.AsRaw(), input.AsRaw())
					require.Equal(t, 1, sets)
				})
			}
		}
	}
}

func BenchmarkIsMatchLiteral(b *testing.B) {
	for _, tc := range []struct {
		name  string
		input string
	}{
		{name: "no-match", input: "no matching literal"},
		{name: "near-miss", input: "workeX-42"},
		{name: "match", input: "worker-42"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			pattern, err := ottl.NewTestingLiteralGetter[any, string](true,
				ottl.StandardStringGetter[any]{Getter: func(context.Context, any) (any, error) {
					return "worker", nil
				}})
			require.NoError(b, err)
			function, err := isMatch[any](
				ottl.StandardStringLikeGetter[any]{Getter: func(context.Context, any) (any, error) {
					return tc.input, nil
				}}, pattern,
			)
			require.NoError(b, err)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := function(b.Context(), nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMatchingKeysLiteral(b *testing.B) {
	for _, keep := range []bool{false, true} {
		name, text := "delete-no-match", "missing"
		if keep {
			name, text = "keep-all-match", "attribute"
		}
		for _, size := range []int{8, 32, 128} {
			b.Run(fmt.Sprintf("%s/%d", name, size), func(b *testing.B) {
				input := pcommon.NewMap()
				for index := range size {
					input.PutInt(fmt.Sprintf("attribute.%03d", index), int64(index))
				}
				pattern, err := ottl.NewTestingLiteralGetter[pcommon.Map, string](true,
					ottl.StandardStringGetter[pcommon.Map]{Getter: func(context.Context, pcommon.Map) (any, error) {
						return text, nil
					}})
				require.NoError(b, err)
				target := ottl.StandardPMapGetSetter[pcommon.Map]{
					Getter: func(_ context.Context, value pcommon.Map) (pcommon.Map, error) { return value, nil },
					Setter: func(context.Context, pcommon.Map, any) error { return nil },
				}
				factory := deleteMatchingKeys[pcommon.Map]
				if keep {
					factory = keepMatchingKeys[pcommon.Map]
				}
				function, err := factory(target, pattern)
				require.NoError(b, err)
				b.ReportAllocs()
				for b.Loop() {
					if _, err := function(b.Context(), input); err != nil {
						b.Fatal(err)
					}
				}
				if input.Len() != size {
					b.Fatal("benchmark input changed")
				}
			})
		}
	}
}
