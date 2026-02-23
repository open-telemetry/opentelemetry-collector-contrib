// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_parseSeverity(t *testing.T) {
	tests := []struct {
		name           string
		target         ottl.Getter[any]
		mapping        func() any
		expected       string
		expectErrorMsg string
	}{
		{
			name: "map from status code - error level",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping:  getTestingGetter,
			expected: "error",
		},
		{
			name: "map from status code - debug level",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(100), nil
				},
			},
			mapping:  getTestingGetter,
			expected: "debug",
		},
		{
			name: "map from status code based on value range",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(200), nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						mapping := m.PutEmptySlice("info")
						mapping.AppendEmpty().SetEmptyMap().PutStr("range", "2xx")
						return m, nil
					},
				})

				return m
			},
			expected: "info",
		},
		{
			name: "map from log level string",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "inf", nil
				},
			},
			mapping:  getTestingGetter,
			expected: "info",
		},
		{
			name: "map from code that matches http status range",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(200), nil
				},
			},
			mapping:  getTestingGetter,
			expected: "info",
		},
		{
			name: "map from log level string, multiple criteria of mixed types defined",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "inf", nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s1 := m.PutEmptySlice("error")
						rangeMap := s1.AppendEmpty().SetEmptyMap()
						rangeMap.PutInt("min", 400)
						rangeMap.PutInt("max", 599)

						s2 := m.PutEmptySlice("info")
						equalsSlice := s2.AppendEmpty().SetEmptyMap().PutEmptySlice("equals")
						equalsSlice.AppendEmpty().SetStr("info")
						equalsSlice.AppendEmpty().SetStr("inf")

						return m, nil
					},
				})
				return m
			},
			expected: "info",
		},
		{
			name: "invalid mapping format",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "foo", nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s := m.PutEmptySlice("info")
						s.AppendEmpty().SetStr("info")
						s.AppendEmpty().SetStr("inf")

						return m, nil
					},
				})

				return m
			},
			expectErrorMsg: "severity mapping criteria items must be map[string]any",
		},
		{
			name: "unexpected type in criteria",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						m.PutStr("error", "invalid")
						return m, nil
					},
				})

				return m
			},
			expectErrorMsg: "severity mapping criteria must be []any",
		},
		{
			name: "unexpected type in range criteria (min), no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s := m.PutEmptySlice("error")
						rangeMap := s.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
						rangeMap.PutStr("min", "foo")
						rangeMap.PutInt("max", 599)

						return m, nil
					},
				})
				return m
			},
			expectErrorMsg: "invalid severity mapping criteria: min must be an int64",
		},
		{
			name: "unexpected type in target, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return map[string]any{"foo": "bar"}, nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s := m.PutEmptySlice("warn")
						rangeMap := s.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
						rangeMap.PutInt("min", 400)
						rangeMap.PutInt("max", 499)

						return m, nil
					},
				})
				return m
			},
			expectErrorMsg: "could not map log level: could not evaluate log level of value 'map[foo:bar]",
		},
		{
			name: "error in acquiring target, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, errors.New("oops")
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s := m.PutEmptySlice("warn")
						rangeMap := s.AppendEmpty().SetEmptyMap()
						rangeMap.PutInt("min", 400)
						rangeMap.PutInt("max", 499)

						return m, nil
					},
				})
				return m
			},
			expectErrorMsg: "could not get log level: oops",
		},
		{
			name: "unexpected type in range criteria (max), no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s := m.PutEmptySlice("error")
						rangeMap := s.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
						rangeMap.PutInt("min", 400)
						rangeMap.PutStr("max", "foo")

						return m, nil
					},
				})
				return m
			},
			expectErrorMsg: "invalid severity mapping criteria: max must be an int64",
		},
		{
			name: "missing min in range, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s := m.PutEmptySlice("error")
						rangeMap := s.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
						rangeMap.PutInt("max", 599)

						return m, nil
					},
				})
				return m
			},
			expectErrorMsg: "invalid severity mapping criteria: range must have a min value",
		},
		{
			name: "missing max in range, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: func() any {
				m, _ := ottl.NewTestingLiteralGetter(true, ottl.StandardPMapGetter[any]{
					Getter: func(_ context.Context, _ any) (any, error) {
						m := pcommon.NewMap()
						s := m.PutEmptySlice("error")
						rangeMap := s.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
						rangeMap.PutInt("min", 400)

						return m, nil
					},
				})
				return m
			},
			expectErrorMsg: "invalid severity mapping criteria: range must have a max value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseSeverity[any](tt.target, tt.mapping().(ottl.PMapGetter[any]))

			result, err := exprFunc(t.Context(), nil)
			if tt.expectErrorMsg != "" {
				assert.ErrorContains(t, err, tt.expectErrorMsg)
				return
			}

			require.NoError(t, err)

			resultString, ok := result.(string)
			require.True(t, ok)

			assert.Equal(t, tt.expected, resultString)
		})
	}
}

func getTestingGetter() any {
	getter, _ := ottl.NewTestingLiteralGetter(
		true,
		ottl.StandardPMapGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return getTestSeverityMapping(), nil
			},
		},
	)
	return getter
}

func getTestSeverityMapping() pcommon.Map {
	m := pcommon.NewMap()
	errorMapping := m.PutEmptySlice("error")
	rangeMap := errorMapping.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
	rangeMap.PutInt("min", 400)
	rangeMap.PutInt("max", 499)

	debugMapping := m.PutEmptySlice("debug")
	rangeMap2 := debugMapping.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
	rangeMap2.PutInt("min", 100)
	rangeMap2.PutInt("max", 199)

	infoMapping := m.PutEmptySlice("info")
	infoEquals := infoMapping.AppendEmpty().SetEmptyMap().PutEmptySlice("equals")
	infoEquals.AppendEmpty().SetStr("inf")
	infoEquals.AppendEmpty().SetStr("info")
	infoMapping.AppendEmpty().SetEmptyMap().PutStr("range", http2xx)

	warnMapping := m.PutEmptySlice("warn")
	rangeMap4 := warnMapping.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
	rangeMap4.PutInt("min", 300)
	rangeMap4.PutInt("max", 399)

	fatalMapping := m.PutEmptySlice("fatal")
	rangeMap5 := fatalMapping.AppendEmpty().SetEmptyMap().PutEmptyMap("range")
	rangeMap5.PutInt("min", 500)
	rangeMap5.PutInt("max", 599)

	return m
}
