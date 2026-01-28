// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"context"
)

type literal[K any, T any] struct {
	value T
}

func newLiteral[K, T any](value T) *literal[K, T] {
	return &literal[K, T]{value: value}
}

func (l *literal[K, T]) Get(context.Context, K) (T, error) {
	return l.value, nil
}

// GetLiteralValue retrieves the literal value from the given getter.
// If the getter is not a literal getter, or if the value it's currently holding is not a
// literal value, it returns the zero value of V and false.
func GetLiteralValue[K, V any](getter typedGetter[K, V]) (V, bool) {
	lit, isLiteral := getter.(*literal[K, V])
	if !isLiteral {
		return *new(V), false
	}

	return lit.value, true
}
