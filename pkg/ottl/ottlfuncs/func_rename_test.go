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
	inputMap := map[string]any{
		"test":  "hello world",
		"test2": int64(3),
		"empty": nil,
	}

	input := pcommon.NewMap()
	err := input.FromRaw(inputMap)
	if err != nil {
		t.Fatal(err)
	}

	target := func(key string) *ottl.StandardGetSetter[pcommon.Value] {
		return &ottl.StandardGetSetter[pcommon.Value]{
			Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
				if val == nil {
					tCtx.Map().PutEmpty(key)
					return nil
				}
				switch v := val.(type) {
				case int64:
					tCtx.Map().PutInt(key, v)
				case string:
					tCtx.Map().PutStr(key, v)
				default:
					t.Fatal("unexpected type")
				}
				return nil
			},
			Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
				return tCtx, nil
			},
		}
	}

	sourceMap := ottl.StandardPMapGetter[pcommon.Value]{
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name             string
		target           ottl.GetSetter[pcommon.Value]
		sourceMap        ottl.PMapGetter[pcommon.Value]
		sourceKey        string
		ignoreMissing    ottl.Optional[bool]
		conflictStrategy ottl.Optional[string]
		want             map[string]any
		wantErr          error
	}{
		{
			name:      "rename test",
			target:    target("test3"),
			sourceMap: sourceMap,
			sourceKey: "test",
			want: map[string]any{
				"test2": int64(3),
				"test3": "hello world",
				"empty": nil,
			},
		},
		{
			name:      "rename empty",
			target:    target("empty3"),
			sourceMap: sourceMap,
			sourceKey: "empty",
			want: map[string]any{
				"test":   "hello world",
				"test2":  int64(3),
				"empty3": nil,
			},
		},
		{
			name:      "rename ignore missing default",
			target:    target("test3"),
			sourceMap: sourceMap,
			sourceKey: "test5",
			want:      inputMap,
		},
		{
			name:          "rename ignore missing true",
			target:        target("test3"),
			sourceMap:     sourceMap,
			sourceKey:     "test5",
			ignoreMissing: ottl.NewTestingOptional[bool](true),
			want:          inputMap,
		},
		{
			name:          "rename ignore missing false",
			target:        target("test3"),
			sourceMap:     sourceMap,
			sourceKey:     "test5",
			ignoreMissing: ottl.NewTestingOptional[bool](false),
			wantErr:       ErrRenameKeyIsMissing,
		},
		{
			name:      "rename with default strategy",
			target:    target("test2"),
			sourceMap: sourceMap,
			sourceKey: "test",
			want: map[string]any{
				"test2": "hello world",
				"empty": nil,
			},
		},
		{
			name:             "rename with upsert strategy",
			target:           target("test2"),
			sourceMap:        sourceMap,
			sourceKey:        "test",
			conflictStrategy: ottl.NewTestingOptional[string](renameConflictUpsert),
			want: map[string]any{
				"test2": "hello world",
				"empty": nil,
			},
		},
		{
			name:             "rename with fail strategy",
			target:           target("test2"),
			sourceMap:        sourceMap,
			sourceKey:        "test",
			conflictStrategy: ottl.NewTestingOptional[string](renameConflictFail),
			wantErr:          ErrRenameKeyAlreadyExists,
		},
		{
			name:             "rename with insert strategy",
			target:           target("test2"),
			sourceMap:        sourceMap,
			sourceKey:        "test",
			conflictStrategy: ottl.NewTestingOptional[string](renameConflictInsert),
			want:             inputMap,
		},
		{
			name:      "rename same key with default strategy",
			target:    target("test"),
			sourceMap: sourceMap,
			sourceKey: "test",
			want:      inputMap,
		},
		{
			name:             "rename same key with upsert strategy",
			target:           target("test"),
			sourceMap:        sourceMap,
			sourceKey:        "test",
			conflictStrategy: ottl.NewTestingOptional[string](renameConflictUpsert),
			want:             inputMap,
		},
		{
			name:             "rename same key with fail strategy",
			target:           target("test"),
			sourceMap:        sourceMap,
			sourceKey:        "test",
			conflictStrategy: ottl.NewTestingOptional[string](renameConflictFail),
			wantErr:          ErrRenameKeyAlreadyExists,
		},
		{
			name:             "rename same key with insert strategy",
			target:           target("test"),
			sourceMap:        sourceMap,
			sourceKey:        "test",
			conflictStrategy: ottl.NewTestingOptional[string](renameConflictInsert),
			want:             inputMap,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			scenarioValue := pcommon.NewValueMap()
			input.CopyTo(scenarioValue.Map())

			exprFunc, err := rename(tt.target, tt.sourceMap, tt.sourceKey, tt.ignoreMissing, tt.conflictStrategy)
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioValue)
			assert.ErrorIs(t, err, tt.wantErr)
			if err != nil {
				return
			}
			assert.Equal(t, tt.want, scenarioValue.Map().AsRaw())
		})
	}
}

func Test_rename_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx, nil
		},
	}

	sourceMap := ottl.StandardPMapGetter[pcommon.Value]{
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	exprFunc, err := rename(target, sourceMap, key, ottl.Optional[bool]{}, ottl.Optional[string]{})
	assert.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_rename_bad_conflict_strategy(t *testing.T) {
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx, nil
		},
	}

	sourceMap := ottl.StandardPMapGetter[pcommon.Value]{
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	_, err := rename(target, sourceMap, key, ottl.Optional[bool]{}, ottl.NewTestingOptional[string]("unsupported"))
	assert.ErrorIs(t, err, ErrRenameInvalidConflictStrategy)
}

func Test_rename_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx, nil
		},
	}

	sourceMap := ottl.StandardPMapGetter[pcommon.Value]{
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	exprFunc, err := rename(target, sourceMap, key, ottl.Optional[bool]{}, ottl.Optional[string]{})
	assert.NoError(t, err)

	_, err = exprFunc(nil, pcommon.NewValueEmpty())
	assert.Error(t, err)
}
