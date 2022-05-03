// Copyright  The OpenTelemetry Authors
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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type TransformContext interface {
	GetItem() interface{}
	GetInstrumentationScope() pcommon.InstrumentationScope
	GetResource() pcommon.Resource
	GetDescriptor() interface{}
}

type ExprFunc func(ctx TransformContext) interface{}

type Getter interface {
	Get(ctx TransformContext) interface{}
}

type Setter interface {
	Set(ctx TransformContext, val interface{})
}

type GetSetter interface {
	Getter
	Setter
}

type literal struct {
	value interface{}
}

func (l literal) Get(ctx TransformContext) interface{} {
	return l.value
}

type exprGetter struct {
	expr ExprFunc
}

func (g exprGetter) Get(ctx TransformContext) interface{} {
	return g.expr(ctx)
}

func NewGetter(val Value, functions map[string]interface{}, pathParser PathExpressionParser) (Getter, error) {
	if s := val.String; s != nil {
		return &literal{value: *s}, nil
	}
	if f := val.Float; f != nil {
		return &literal{value: *f}, nil
	}
	if i := val.Int; i != nil {
		return &literal{value: *i}, nil
	}

	if val.Path != nil {
		return pathParser(val.Path)
	}

	if val.Invocation == nil {
		// In practice, can't happen since the DSL grammar guarantees one is set
		return nil, fmt.Errorf("no value field set. This is a bug in the transformprocessor")
	}
	call, err := NewFunctionCall(*val.Invocation, functions, pathParser)
	if err != nil {
		return nil, err
	}
	return &exprGetter{
		expr: call,
	}, nil
}
