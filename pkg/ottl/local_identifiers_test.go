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

func TestLocalBindingScopeStack(t *testing.T) {
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

func Test_newLocalBindingGetter(t *testing.T) {
	tests := []struct {
		name             string
		identifierPath   *localIdentifier
		localScopeFrames []localScopeFrame
		want             Getter[any]
		wantErr          string
	}{
		{
			name:             "valid",
			identifierPath:   &localIdentifier{Name: "$value"},
			localScopeFrames: []localScopeFrame{{"$value": {}}},
			want:             &localBindingGetter[any]{identifierPath: &localIdentifier{Name: "$value"}},
		},
		{
			name:           "local identifier outside scoped body",
			identifierPath: &localIdentifier{Name: "$value"},
			wantErr:        "local identifier $value is only valid inside a scoped body",
		},
		{
			name:             "unknown local identifier",
			identifierPath:   &localIdentifier{Name: "$foo"},
			localScopeFrames: []localScopeFrame{{"$bar": {}}},
			wantErr:          "local identifier $foo is not in scope",
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

func TestLocalBindingGetter_Get(t *testing.T) {
	tests := []struct {
		name    string
		getter  *localBindingGetter[any]
		ctx     context.Context
		want    any
		wantErr string
	}{
		{
			name: "outside binding scope",
			getter: &localBindingGetter[any]{
				identifierPath: &localIdentifier{Name: localIdentifierDecl("$missing")},
			},
			ctx:     t.Context(),
			wantErr: "local identifier $missing evaluated outside of a local binding scope",
		},
		{
			name: "missing binding",
			getter: &localBindingGetter[any]{
				identifierPath: &localIdentifier{Name: localIdentifierDecl("$missing")},
			},
			ctx:     context.WithValue(t.Context(), localBindingsKey{}, map[string]any{"other": 1}),
			wantErr: "missing value for local identifier $missing",
		},
		{
			name: "returns direct binding",
			getter: &localBindingGetter[any]{
				identifierPath: &localIdentifier{Name: localIdentifierDecl("$value")},
			},
			ctx:  context.WithValue(t.Context(), localBindingsKey{}, map[string]any{"$value": "ok"}),
			want: "ok",
		},
		{
			name: "returns indexed binding",
			getter: &localBindingGetter[any]{
				identifierPath: &localIdentifier{
					Name: localIdentifierDecl("$value"),
					Keys: []key{{String: ottltest.Strp("field")}},
				},
			},
			ctx:  context.WithValue(t.Context(), localBindingsKey{}, map[string]any{"$value": map[string]any{"field": "ok"}}),
			want: "ok",
		},
		{
			name: "indexing error is wrapped",
			getter: &localBindingGetter[any]{
				identifierPath: &localIdentifier{
					Name: localIdentifierDecl("$value"),
					Keys: []key{{Int: ottltest.Intp(2)}},
				},
			},
			ctx:     context.WithValue(t.Context(), localBindingsKey{}, map[string]any{"$value": []any{"only"}}),
			wantErr: "cannot index local identifier $value: index 2 out of bounds",
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

func makeLocalIdentifiers(args ...string) []LocalIdentifier {
	res := make([]LocalIdentifier, len(args))
	for i, v := range args {
		lid := localIdentifierDecl(v)
		res[i] = &lid
	}
	return res
}
