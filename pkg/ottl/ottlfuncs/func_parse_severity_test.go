// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
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
		mapping        ottl.PMapGetter[any]
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
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getTestSeverityMapping(), nil
				},
			},
			expected: "error",
		},
		{
			name: "map from status code - debug level",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(100), nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getTestSeverityMapping(), nil
				},
			},
			expected: "debug",
		},
		{
			name: "map from status code based on value range",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(200), nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					mapping := m.PutEmptySlice("info")
					mapping.AppendEmpty().SetStr("2xx")
					return m, nil
				},
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
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return getTestSeverityMapping(), nil
				},
			},
			expected: "info",
		},
		{
			name: "map from log level string, multiple criteria of mixed types defined",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "inf", nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s1 := m.PutEmptySlice("error")
					rangeMap := s1.AppendEmpty().SetEmptyMap()
					rangeMap.PutInt("min", 400)
					rangeMap.PutInt("max", 599)

					s2 := m.PutEmptySlice("info")
					s2.AppendEmpty().SetStr("info")
					s2.AppendEmpty().SetStr("inf")

					return m, nil
				},
			},
			expected: "info",
		},
		{
			name: "no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "foo", nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s := m.PutEmptySlice("info")
					s.AppendEmpty().SetStr("info")
					s.AppendEmpty().SetStr("inf")

					return m, nil
				},
			},
			expectErrorMsg: "could not map log level: no matching log level found for value 'foo'",
		},
		{
			name: "unexpected type in range criteria (min), no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s := m.PutEmptySlice("error")
					rangeMap := s.AppendEmpty().SetEmptyMap()
					rangeMap.PutStr("min", "foo")
					rangeMap.PutInt("max", 599)

					return m, nil
				},
			},
			expectErrorMsg: "could not map log level: no matching log level found for value '400'",
		},
		{
			name: "unexpected type in target, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return map[string]any{"foo": "bar"}, nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s := m.PutEmptySlice("warn")
					rangeMap := s.AppendEmpty().SetEmptyMap()
					rangeMap.PutInt("min", 400)
					rangeMap.PutInt("max", 499)

					return m, nil
				},
			},
			expectErrorMsg: "log level must be either string or int64, but got map[string]interface {}",
		},
		{
			name: "error in acquiring target, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, fmt.Errorf("oops")
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s := m.PutEmptySlice("warn")
					rangeMap := s.AppendEmpty().SetEmptyMap()
					rangeMap.PutInt("min", 400)
					rangeMap.PutInt("max", 499)

					return m, nil
				},
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
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s := m.PutEmptySlice("error")
					rangeMap := s.AppendEmpty().SetEmptyMap()
					rangeMap.PutInt("min", 400)
					rangeMap.PutStr("max", "foo")

					return m, nil
				},
			},
			expectErrorMsg: "could not map log level: no matching log level found for value '400'",
		},
		{
			name: "missing min in range, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s := m.PutEmptySlice("error")
					rangeMap := s.AppendEmpty().SetEmptyMap()
					rangeMap.PutInt("max", 599)

					return m, nil
				},
			},
			expectErrorMsg: "could not map log level: no matching log level found for value '400'",
		},
		{
			name: "missing max in range, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					m := pcommon.NewMap()
					s := m.PutEmptySlice("error")
					rangeMap := s.AppendEmpty().SetEmptyMap()
					rangeMap.PutInt("min", 400)

					return m, nil
				},
			},
			expectErrorMsg: "could not map log level: no matching log level found for value '400'",
		},
		{
			name: "incorrect format of severity mapping, no match",
			target: ottl.StandardGetSetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(400), nil
				},
			},
			mapping: ottl.StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "invalid", nil
				},
			},
			expectErrorMsg: "cannot get severity mapping: expected pcommon.Map but got string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseSeverity[any](tt.target, tt.mapping)

			result, err := exprFunc(context.Background(), nil)
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

func getTestSeverityMapping() pcommon.Map {
	m := pcommon.NewMap()
	errorMapping := m.PutEmptySlice("error")
	rangeMap := errorMapping.AppendEmpty().SetEmptyMap()
	rangeMap.PutInt("min", 400)
	rangeMap.PutInt("max", 499)

	debugMapping := m.PutEmptySlice("debug")
	rangeMap2 := debugMapping.AppendEmpty().SetEmptyMap()
	rangeMap2.PutInt("min", 100)
	rangeMap2.PutInt("max", 199)

	infoMapping := m.PutEmptySlice("info")
	infoMapping.AppendEmpty().SetStr("inf")
	infoMapping.AppendEmpty().SetStr("info")
	rangeMap3 := infoMapping.AppendEmpty().SetEmptyMap()
	rangeMap3.PutInt("min", 200)
	rangeMap3.PutInt("max", 299)

	warnMapping := m.PutEmptySlice("warn")
	rangeMap4 := warnMapping.AppendEmpty().SetEmptyMap()
	rangeMap4.PutInt("min", 300)
	rangeMap4.PutInt("max", 399)

	fatalMapping := m.PutEmptySlice("fatal")
	rangeMap5 := fatalMapping.AppendEmpty().SetEmptyMap()
	rangeMap5.PutInt("min", 500)
	rangeMap5.PutInt("max", 599)

	return m
}

func Test_parseValueRangePlaceholder(t *testing.T) {
	type args struct {
		crit any
	}
	tests := []struct {
		name        string
		args        args
		wantMapping map[string]any
		wantOk      bool
	}{
		{
			name: "2xx",
			args: args{
				crit: hTTP2xx,
			},
			wantMapping: map[string]any{
				"min": int64(200),
				"max": int64(299),
			},
			wantOk: true,
		},
		{
			name: "3xx",
			args: args{
				crit: hTTP3xx,
			},
			wantMapping: map[string]any{
				"min": int64(300),
				"max": int64(399),
			},
			wantOk: true,
		},
		{
			name: "4xx",
			args: args{
				crit: hTTP4xx,
			},
			wantMapping: map[string]any{
				"min": int64(400),
				"max": int64(499),
			},
			wantOk: true,
		},
		{
			name: "5xx",
			args: args{
				crit: hTTP5xx,
			},
			wantMapping: map[string]any{
				"min": int64(500),
				"max": int64(599),
			},
			wantOk: true,
		},
		{
			name: "unknown",
			args: args{
				crit: "unknown",
			},
			wantMapping: nil,
			wantOk:      false,
		},
		{
			name: "not a string",
			args: args{
				crit: 1,
			},
			wantMapping: nil,
			wantOk:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := parseValueRangePlaceholder(tt.args.crit)
			assert.Equalf(t, tt.wantMapping, got, "parseValueRangePlaceholder(%v)", tt.args.crit)
			assert.Equalf(t, tt.wantOk, got1, "parseValueRangePlaceholder(%v)", tt.args.crit)
		})
	}
}
