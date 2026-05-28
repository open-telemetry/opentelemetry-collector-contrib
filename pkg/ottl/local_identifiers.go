// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
	"fmt"
	"maps"
	"slices"
)

// LocalIdentifier represents a named or blank parameter in a local scope.
// Name returns the identifier's name as a string.
// IsBlank indicates whether the identifier is a blank ("_") placeholder.
//
// Experimental: *NOTE* this API is subject to change or removal in the future.
type LocalIdentifier interface {
	// Name returns the identifier's name as a string.
	Name() string
	// IsBlank indicates whether the identifier is a blank ("_") placeholder.
	IsBlank() bool
}

// localBindingsKey is a context key used for storing local identifier bindings at evaluation time.
type localBindingsKey struct{}

// localScopeFrame is the set of local identifier lexemes declared in one scope frame (parse time only).
type localScopeFrame map[string]struct{}

// localScopeStack tracks which names are in scope while parsing a subtree (parse time only).
// Lambdas and other constructs that introduce local identifiers push and pop frames on this stack.
// Frames record only local declarations; context data paths are not tracked here and are still
// resolved through PathExpressionParser like any other path.
type localScopeStack []localScopeFrame

func (s *localScopeStack) push(frame localScopeFrame) {
	*s = append(*s, frame)
}

func (s *localScopeStack) pop() {
	*s = (*s)[:len(*s)-1]
}

func (s *localScopeStack) empty() bool {
	return len(*s) == 0
}

func (s *localScopeStack) inScope(lexeme string) bool {
	if s.empty() {
		return false
	}
	for _, v := range slices.Backward(*s) {
		if _, ok := v[lexeme]; ok {
			return true
		}
	}
	return false
}

// withLocalScope runs fn while the frame is on the parseContext.localScopes stack.
func (p *parseContext[K]) withLocalScope(frame localScopeFrame, fn func() error) error {
	p.localScopes.push(frame)
	defer p.localScopes.pop()
	return fn()
}

// withLocalBindings merges parent and new bindings into ctx, then runs fn.
func withLocalBindings(ctx context.Context, bindings map[string]any, fn func(context.Context) (any, error)) (any, error) {
	merged := make(map[string]any)
	if parent, ok := ctx.Value(localBindingsKey{}).(map[string]any); ok {
		maps.Copy(merged, parent)
	}
	maps.Copy(merged, bindings)
	return fn(context.WithValue(ctx, localBindingsKey{}, merged))
}

type localBindingGetter[K any] struct {
	identifierPath *localIdentifier
}

func (p *parseContext[K]) newLocalIdentifierGetter(identifierPath *localIdentifier) (Getter[K], error) {
	name := string(identifierPath.Name)
	if p.localScopes.empty() {
		return nil, fmt.Errorf("local identifier %s is only valid inside a scoped body", name)
	}
	if !p.localScopes.inScope(name) {
		return nil, fmt.Errorf("local identifier %s is not in scope", name)
	}
	return &localBindingGetter[K]{identifierPath: identifierPath}, nil
}

func (g *localBindingGetter[K]) Get(ctx context.Context, tCtx K) (any, error) {
	bindings, ok := ctx.Value(localBindingsKey{}).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("local identifier %s evaluated outside of a local binding scope", g.identifierPath.Name)
	}
	v, ok := bindings[string(g.identifierPath.Name)]
	if !ok {
		return nil, fmt.Errorf("missing value for local identifier %s", g.identifierPath.Name)
	}
	if len(g.identifierPath.Keys) > 0 {
		return g.getIndexableValue(ctx, tCtx, v)
	}
	return v, nil
}

func (g *localBindingGetter[K]) getIndexableValue(ctx context.Context, tCtx K, val any) (any, error) {
	getter := &exprGetter[K]{
		expr: Expr[K]{exprFunc: func(context.Context, K) (any, error) {
			return val, nil
		}},
		keys: g.identifierPath.Keys,
	}

	result, err := getter.Get(ctx, tCtx)
	if err != nil {
		return nil, fmt.Errorf("cannot index local identifier %s: %w", g.identifierPath.Name, err)
	}

	return result, nil
}
