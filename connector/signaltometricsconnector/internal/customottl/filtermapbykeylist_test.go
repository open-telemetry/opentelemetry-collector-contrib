// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package customottl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type testGetter[K any] struct {
	val any
	err error
}

func (g *testGetter[K]) Get(_ context.Context, _ K) (any, error) {
	return g.val, g.err
}

type testStringGetter[K any] struct {
	val string
}

func (g *testStringGetter[K]) Get(_ context.Context, _ K) (string, error) {
	return g.val, nil
}

type testPMapGetter[K any] struct {
	m pcommon.Map
}

func (g *testPMapGetter[K]) Get(_ context.Context, _ K) (pcommon.Map, error) {
	return g.m, nil
}

func TestFilterMapByKeyList(t *testing.T) {
	cases := []struct {
		name     string
		source   map[string]any
		keyList  any
		prefixes []string
		want     map[string]any
	}{
		{
			name: "basic_filtering",
			source: map[string]any{
				"labels.team":         "platform",
				"labels.env":          "prod",
				"numeric_labels.cost": 42,
				"service.name":        "svc",
			},
			keyList:  "team,cost",
			prefixes: []string{"labels.", "numeric_labels."},
			want: map[string]any{
				"labels.team":         "platform",
				"numeric_labels.cost": int64(42),
			},
		},
		{
			name: "empty_key_list",
			source: map[string]any{
				"labels.team": "platform",
			},
			keyList:  "",
			prefixes: []string{"labels."},
			want:     map[string]any{},
		},
		{
			name: "nil_key_list",
			source: map[string]any{
				"labels.team": "platform",
			},
			keyList:  nil,
			prefixes: []string{"labels."},
			want:     map[string]any{},
		},
		{
			name: "whitespace_in_keys",
			source: map[string]any{
				"labels.team": "platform",
				"labels.env":  "prod",
			},
			keyList:  " team , env ",
			prefixes: []string{"labels."},
			want: map[string]any{
				"labels.team": "platform",
				"labels.env":  "prod",
			},
		},
		{
			name: "no_matching_prefix",
			source: map[string]any{
				"service.name": "svc",
				"host.name":    "h1",
			},
			keyList:  "name",
			prefixes: []string{"labels."},
			want:     map[string]any{},
		},
		{
			name: "wildcard_accepts_all_prefixed",
			source: map[string]any{
				"labels.team":         "platform",
				"labels.env":          "prod",
				"numeric_labels.cost": 42,
				"service.name":        "svc",
			},
			keyList:  "*",
			prefixes: []string{"labels.", "numeric_labels."},
			want: map[string]any{
				"labels.team":         "platform",
				"labels.env":          "prod",
				"numeric_labels.cost": int64(42),
			},
		},
		{
			name: "wildcard_no_matching_prefix",
			source: map[string]any{
				"service.name": "svc",
				"host.name":    "h1",
			},
			keyList:  "*",
			prefixes: []string{"labels."},
			want:     map[string]any{},
		},
		{
			name: "pcommon_slice_key_list",
			source: map[string]any{
				"labels.team": "platform",
				"labels.env":  "prod",
			},
			keyList: func() pcommon.Slice {
				s := pcommon.NewSlice()
				s.AppendEmpty().SetStr("team")
				return s
			}(),
			prefixes: []string{"labels."},
			want: map[string]any{
				"labels.team": "platform",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcMap := pcommon.NewMap()
			require.NoError(t, srcMap.FromRaw(tc.source))

			prefixGetters := make([]ottl.StringGetter[any], len(tc.prefixes))
			for i, p := range tc.prefixes {
				prefixGetters[i] = &testStringGetter[any]{val: p}
			}

			fn := filterMapByKeyList(
				&testPMapGetter[any]{m: srcMap},
				&testGetter[any]{val: tc.keyList},
				prefixGetters,
			)

			result, err := fn(context.Background(), nil)
			require.NoError(t, err)

			resultMap, ok := result.(pcommon.Map)
			require.True(t, ok)
			require.Equal(t, len(tc.want), resultMap.Len())

			for k, v := range tc.want {
				got, exists := resultMap.Get(k)
				require.True(t, exists, "expected key %q", k)
				require.Equal(t, v, got.AsRaw())
			}
		})
	}
}

func TestCreateFilterMapByKeyListFunction_Errors(t *testing.T) {
	cases := []struct {
		name    string
		args    ottl.Arguments
		wantErr string
	}{
		{
			name:    "wrong_arg_type",
			args:    &struct{}{},
			wantErr: "FilterMapByKeyList args must be of type *FilterMapByKeyListArguments[K]",
		},
		{
			name:    "no_prefixes",
			args:    &FilterMapByKeyListArguments[any]{},
			wantErr: "FilterMapByKeyList requires at least one prefix",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := createFilterMapByKeyListFunction[any](ottl.FunctionContext{}, tc.args)
			require.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestParseKeyList(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want map[string]struct{}
	}{
		{name: "empty", raw: "", want: nil},
		{name: "single", raw: "team", want: map[string]struct{}{"team": {}}},
		{name: "multiple", raw: "team,cost", want: map[string]struct{}{"team": {}, "cost": {}}},
		{name: "whitespace", raw: " team , cost ", want: map[string]struct{}{"team": {}, "cost": {}}},
		{name: "trailing_comma", raw: "team,", want: map[string]struct{}{"team": {}}},
		{name: "wildcard", raw: "*", want: map[string]struct{}{"*": {}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseKeyList(tc.raw)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolveKeyListString(t *testing.T) {
	cases := []struct {
		name string
		val  any
		want string
	}{
		{name: "string", val: "team,cost", want: "team,cost"},
		{name: "nil", val: nil, want: ""},
		{
			name: "pcommon_slice",
			val: func() pcommon.Slice {
				s := pcommon.NewSlice()
				s.AppendEmpty().SetStr("team,cost")
				return s
			}(),
			want: "team,cost",
		},
		{
			name: "empty_slice",
			val:  pcommon.NewSlice(),
			want: "",
		},
		{name: "int_fallback", val: 42, want: "42"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveKeyListString(tc.val)
			require.Equal(t, tc.want, got)
		})
	}
}
