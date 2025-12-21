// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package runtime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/runtime"

import (
	"context"
)

type literal[K any, V any] struct {
	value V
}

func NewLiteral[K, V any](value V) Getter[K, V] {
	return &literal[K, V]{value: value}
}

func (l *literal[K, V]) Get(context.Context, K) (V, error) {
	return l.value, nil
}

func (*literal[K, V]) isLiteral() {}

// GetLiteralValue retrieves the literal value from the given getter.
// If the getter is not a literal getter, or if the value it's currently holding is not a
// literal value, it returns the zero value of V and false.
func GetLiteralValue[K, V any](getter Getter[K, V]) (V, bool) {
	l, isLiteral := getter.(*literal[K, V])
	if !isLiteral {
		return *new(V), false
	}
	return l.value, true
}
