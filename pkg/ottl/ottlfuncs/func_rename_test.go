// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/assert"
)

func Test_rename(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutEmpty("empty")

	mg := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name             string
		mg               ottl.PMapGetter[pcommon.Map]
		field            string
		targetField      string
		ignoreMissing    ottl.Optional[bool]
		conflictStrategy ottl.Optional[string]
		want             func(pcommon.Map)
		wantErr          error
	}{
		{
			name:        "rename test",
			mg:          mg,
			field:       "test",
			targetField: "test3",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test3", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutEmpty("empty")
			},
		},
		{
			name:        "rename empty",
			mg:          mg,
			field:       "empty",
			targetField: "empty3",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutEmpty("empty3")
			},
		},
		{
			name:        "rename ignore missing default",
			mg:          mg,
			field:       "test5",
			targetField: "test3",
			want: func(expectedMap pcommon.Map) {
				input.CopyTo(expectedMap)
			},
		},
		{
			name:          "rename ignore missing true",
			mg:            mg,
			field:         "test5",
			targetField:   "test3",
			ignoreMissing: ottl.NewTestingOptional[bool](true),
			want: func(expectedMap pcommon.Map) {
				input.CopyTo(expectedMap)
			},
		},
		{
			name:          "rename ignore missing false",
			mg:            mg,
			field:         "test5",
			targetField:   "test3",
			ignoreMissing: ottl.NewTestingOptional[bool](false),
			wantErr:       ErrRenameKeyIsMissing,
		},
		{
			name:        "rename with default strategy",
			mg:          mg,
			field:       "test",
			targetField: "test2",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutEmpty("empty")
				expectedMap.PutStr("test2", "hello world")
			},
		},
		{
			name:             "rename with replace strategy",
			mg:               mg,
			field:            "test",
			targetField:      "test2",
			conflictStrategy: ottl.NewTestingOptional[string]("replace"),
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutEmpty("empty")
				expectedMap.PutStr("test2", "hello world")
			},
		},
		{
			name:             "rename with fail strategy",
			mg:               mg,
			field:            "test",
			targetField:      "test2",
			conflictStrategy: ottl.NewTestingOptional[string]("fail"),
			wantErr:          ErrRenameKeyAlreadyExists,
		},
		{
			name:             "rename with ignore strategy",
			mg:               mg,
			field:            "test",
			targetField:      "test2",
			conflictStrategy: ottl.NewTestingOptional[string]("ignore"),
			want: func(expectedMap pcommon.Map) {
				input.CopyTo(expectedMap)
			},
		},
		{
			name:        "rename same key with default strategy",
			mg:          mg,
			field:       "test",
			targetField: "test",
			want: func(expectedMap pcommon.Map) {
				input.CopyTo(expectedMap)
			},
		},
		{
			name:             "rename same key with replace strategy",
			mg:               mg,
			field:            "test",
			targetField:      "test",
			conflictStrategy: ottl.NewTestingOptional[string]("replace"),
			want: func(expectedMap pcommon.Map) {
				input.CopyTo(expectedMap)
			},
		},
		{
			name:             "rename same key with fail strategy",
			mg:               mg,
			field:            "test",
			targetField:      "test",
			conflictStrategy: ottl.NewTestingOptional[string]("fail"),
			wantErr:          ErrRenameKeyAlreadyExists,
		},
		{
			name:             "rename same key with ignore strategy",
			mg:               mg,
			field:            "test",
			targetField:      "test",
			conflictStrategy: ottl.NewTestingOptional[string]("ignore"),
			want: func(expectedMap pcommon.Map) {
				input.CopyTo(expectedMap)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := rename(tt.mg, tt.field, tt.targetField, tt.ignoreMissing, tt.conflictStrategy)
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			assert.ErrorIs(t, err, tt.wantErr)
			if err != nil {
				return
			}

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_rename_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	exprFunc, err := rename[any](target, key, key, ottl.Optional[bool]{}, ottl.Optional[string]{})
	assert.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_rename_bad_conflict_strategy(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	_, err := rename[any](target, key, key, ottl.Optional[bool]{}, ottl.NewTestingOptional[string]("unsupported"))
	assert.ErrorIs(t, err, ErrRenameInvalidConflictStrategy)
}

func Test_rename_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	exprFunc, err := rename[any](target, key, key, ottl.Optional[bool]{}, ottl.Optional[string]{})
	assert.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
