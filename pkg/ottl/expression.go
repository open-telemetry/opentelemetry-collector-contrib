// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"fmt"
)

type ExprFunc[K any] func(ctx K) (interface{}, error)

type Getter[K any] interface {
	Get(ctx K) (interface{}, error)
}

type Setter[K any] interface {
	Set(ctx K, val interface{}) error
}

type GetSetter[K any] interface {
	Getter[K]
	Setter[K]
}

type StandardGetSetter[K any] struct {
	Getter func(ctx K) (interface{}, error)
	Setter func(ctx K, val interface{}) error
}

func (path StandardGetSetter[K]) Get(ctx K) (interface{}, error) {
	return path.Getter(ctx)
}

func (path StandardGetSetter[K]) Set(ctx K, val interface{}) error {
	return path.Setter(ctx, val)
}

type literal[K any] struct {
	value interface{}
}

func (l literal[K]) Get(K) (interface{}, error) {
	return l.value, nil
}

type exprGetter[K any] struct {
	expr ExprFunc[K]
}

func (g exprGetter[K]) Get(ctx K) (interface{}, error) {
	return g.expr(ctx)
}

func (p *Parser[K]) newGetter(val value) (Getter[K], error) {
	if val.IsNil != nil && *val.IsNil {
		return &literal[K]{value: nil}, nil
	}

	if s := val.String; s != nil {
		return &literal[K]{value: *s}, nil
	}
	if f := val.Float; f != nil {
		return &literal[K]{value: *f}, nil
	}
	if i := val.Int; i != nil {
		return &literal[K]{value: *i}, nil
	}
	if b := val.Bool; b != nil {
		return &literal[K]{value: bool(*b)}, nil
	}
	if b := val.Bytes; b != nil {
		return &literal[K]{value: ([]byte)(*b)}, nil
	}

	if val.Enum != nil {
		enum, err := p.enumParser(val.Enum)
		if err != nil {
			return nil, err
		}
		return &literal[K]{value: int64(*enum)}, nil
	}

	if val.Path != nil {
		return p.pathParser(val.Path)
	}

	if val.Invocation == nil {
		// In practice, can't happen since the DSL grammar guarantees one is set
		return nil, fmt.Errorf("no value field set. This is a bug in the OpenTelemetry Transformation Language")
	}
	call, err := p.newFunctionCall(*val.Invocation)
	if err != nil {
		return nil, err
	}
	return &exprGetter[K]{
		expr: call,
	}, nil
}
