// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func TestLocalScopeStack(t *testing.T) {
	var scopes localScopeStack
	assert.True(t, scopes.empty())
	assert.False(t, scopes.inScope("value"))

	scopes.push(localScopeFrame{"outer": {}})
	assert.False(t, scopes.empty())
	assert.True(t, scopes.inScope("outer"))
	assert.False(t, scopes.inScope("inner"))

	scopes.push(localScopeFrame{"inner": {}})
	assert.True(t, scopes.inScope("outer"))
	assert.True(t, scopes.inScope("inner"))

	scopes.pop()
	assert.True(t, scopes.inScope("outer"))
	assert.False(t, scopes.inScope("inner"))

	scopes.pop()
	assert.True(t, scopes.empty())
}

func Test_newLocalIdentifierGetter(t *testing.T) {
	tests := []struct {
		name             string
		identifierPath   *basePath[any]
		localScopeFrames []localScopeFrame
		want             Getter[any]
		wantErr          string
	}{
		{
			name:             "valid",
			identifierPath:   &basePath[any]{name: "value", localIdentifier: true},
			localScopeFrames: []localScopeFrame{{"value": {}}},
			want:             &localIdentifierGetter[any]{identifier: &basePath[any]{name: "value", localIdentifier: true}},
		},
		{
			name:             "invalid",
			identifierPath:   &basePath[any]{name: "value", originalText: "value", localIdentifier: false},
			localScopeFrames: []localScopeFrame{{"value": {}}},
			wantErr:          `"value" is not a valid local identifier`,
		},
		{
			name:           "local identifier outside scoped body",
			identifierPath: &basePath[any]{name: "value", localIdentifier: true},
			wantErr:        `local identifier "value" is only valid inside a scoped context`,
		},
		{
			name:             "unknown local identifier",
			identifierPath:   &basePath[any]{name: "foo", localIdentifier: true},
			localScopeFrames: []localScopeFrame{{"bar": {}}},
			wantErr:          `local identifier "foo" is not defined in the local scope`,
		},
	}

	p, _ := NewParser[any](
		map[string]Factory[any]{},
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := p.newParseContext()
			if len(tt.localScopeFrames) > 0 {
				for _, stack := range tt.localScopeFrames {
					pc.localScopes.push(stack)
				}
			}
			res, err := pc.newLocalIdentifierGetter(tt.identifierPath)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, res)
		})
	}
}

func Test_localIdentifierGetter_Get(t *testing.T) {
	tests := []struct {
		name    string
		getter  *localIdentifierGetter[any]
		ctx     context.Context
		want    any
		wantErr string
	}{
		{
			name: "outside active local scope",
			getter: &localIdentifierGetter[any]{
				identifier: &basePath[any]{name: "missing"},
			},
			ctx:     t.Context(),
			wantErr: `local identifier "missing" evaluated outside of an active local scope`,
		},
		{
			name: "missing binding",
			getter: &localIdentifierGetter[any]{
				identifier: &basePath[any]{name: "missing"},
			},
			ctx:     context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"other": 1}}),
			wantErr: `missing value for local identifier "missing"`,
		},
		{
			name: "returns direct binding",
			getter: &localIdentifierGetter[any]{
				identifier: &basePath[any]{name: "value"},
			},
			ctx:  context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"value": "ok"}}),
			want: "ok",
		},
		{
			name: "returns indexed binding",
			getter: &localIdentifierGetter[any]{
				identifier: &basePath[any]{
					name: "value",
					keys: []Key[any]{
						&baseKey[any]{s: ottltest.Strp("field")},
					},
				},
			},
			ctx:  context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"value": map[string]any{"field": "ok"}}}),
			want: "ok",
		},
		{
			name: "indexing error is wrapped",
			getter: &localIdentifierGetter[any]{
				identifier: &basePath[any]{
					name: "value",
					keys: []Key[any]{&baseKey[any]{i: ottltest.Intp(2)}},
				},
			},
			ctx:     context.WithValue(t.Context(), localActivationKey{}, &localActivation{bindings: map[string]any{"value": []any{"only"}}}),
			wantErr: `cannot index local identifier "value": index 2 out of bounds`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.getter.Get(tt.ctx, nil)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_makeLocalIdentifiers(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		want  []string
		blank []bool
	}{
		{
			name: "no params",
			args: nil,
			want: nil,
		},
		{
			name:  "named params",
			args:  []string{"a", "b", "c"},
			want:  []string{"a", "b", "c"},
			blank: []bool{false, false, false},
		},
		{
			name:  "blank param",
			args:  []string{"_"},
			want:  []string{"_"},
			blank: []bool{true},
		},
		{
			name:  "blank and named params",
			args:  []string{"_", "value", "_"},
			want:  []string{"_", "value", "_"},
			blank: []bool{true, false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeLocalIdentifiers(tt.args...)
			require.Len(t, got, len(tt.want))
			for i, decl := range got {
				assert.Equal(t, tt.want[i], decl.Name())
				assert.Equal(t, tt.blank[i], decl.IsBlank())
			}
		})
	}
}

func Test_resolveLocalIdentifierBinding(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		binding string
		want    any
		wantErr string
	}{
		{
			name:    "outside active local scope",
			ctx:     t.Context(),
			binding: "a",
			wantErr: `local identifier "a" evaluated outside of an active local scope`,
		},
		{
			name: "bound value",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				bindings: map[string]any{"a": 1},
			}),
			binding: "a",
			want:    1,
		},
		{
			name: "missing binding",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				bindings: map[string]any{"a": 1},
			}),
			binding: "missing",
			wantErr: `missing value for local identifier "missing"`,
		},
		{
			name: "inherits from parent activation",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				parent:   &localActivation{bindings: map[string]any{"outer": "parent-value"}},
				bindings: map[string]any{"inner": "child-value"},
			}),
			binding: "outer",
			want:    "parent-value",
		},
		{
			name: "child shadows parent binding",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				parent:   &localActivation{bindings: map[string]any{"value": "parent"}},
				bindings: map[string]any{"value": "child"},
			}),
			binding: "value",
			want:    "child",
		},
		{
			name: "explicit nil binding",
			ctx: context.WithValue(t.Context(), localActivationKey{}, &localActivation{
				bindings: map[string]any{"a": nil},
			}),
			binding: "a",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveLocalIdentifierBinding(tt.ctx, tt.binding)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
