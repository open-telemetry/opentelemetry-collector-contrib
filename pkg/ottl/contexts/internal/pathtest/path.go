// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pathtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var _ ottl.Path[any] = &Path[any]{}

type Path[K any] struct {
	C        string
	N        string
	KeySlice []ottl.Key[K]
	NextPath *Path[K]
	FullPath string
}

func (p *Path[K]) Name() string {
	return p.N
}

func (p *Path[K]) Context() string {
	return p.C
}

func (p *Path[K]) Next() ottl.Path[K] {
	if p.NextPath == nil {
		return nil
	}
	return p.NextPath
}

func (p *Path[K]) Keys() []ottl.Key[K] {
	return p.KeySlice
}

func (p *Path[K]) String() string {
	if p.FullPath != "" {
		return p.FullPath
	}
	return p.N
}

var _ ottl.Key[any] = &Key[any]{}

type Key[K any] struct {
	S *string
	I *int64
	G ottl.Getter[K]
}

func (k *Key[K]) String(_ context.Context, _ K) (*string, error) {
	return k.S, nil
}

func (k *Key[K]) Int(_ context.Context, _ K) (*int64, error) {
	return k.I, nil
}

func (k *Key[K]) ExpressionGetter(_ context.Context, _ K) (ottl.Getter[K], error) {
	return k.G, nil
}
