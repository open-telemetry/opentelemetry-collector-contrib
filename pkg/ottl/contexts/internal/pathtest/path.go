// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pathtest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/pathtest"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlpath"
)

var _ ottlpath.Path[any] = &Path[any]{}

type Path[K any] struct {
	C        string
	N        string
	KeySlice []ottlpath.Key[K]
	NextPath *Path[K]
	FullPath string
}

func (p *Path[K]) Name() string {
	return p.N
}

func (p *Path[K]) Context() string {
	return p.C
}

func (p *Path[K]) Next() ottlpath.Path[K] {
	if p.NextPath == nil {
		return nil
	}
	return p.NextPath
}

func (p *Path[K]) Keys() []ottlpath.Key[K] {
	return p.KeySlice
}

func (p *Path[K]) String() string {
	if p.FullPath != "" {
		return p.FullPath
	}
	return p.N
}

var _ ottlpath.Key[any] = &Key[any]{}

type Key[K any] struct {
	S *string
	I *int64
	G ottlpath.Getter[K, any]
}

func (k *Key[K]) String(_ context.Context, _ K) (*string, error) {
	return k.S, nil
}

func (k *Key[K]) Int(_ context.Context, _ K) (*int64, error) {
	return k.I, nil
}

func (k *Key[K]) ExpressionGetter(_ context.Context, _ K) (ottlpath.Getter[K, any], error) {
	return k.G, nil
}
