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

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"fmt"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type exprFunc func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{}

// getter allows reading a value while processing traces. Note that data is not necessarily read from input
// telemetry but may be a literal value or a function invocation.
type getter interface {
	get(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{}
}

// getSetter allows reading or writing a value to trace data.
type getSetter interface {
	getter
	set(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{})
}

// literal holds a literal value defined as part of a Query. It does not read from telemetry data.
type literal struct {
	value interface{}
}

func (l literal) get(_ pdata.Span, _ pdata.InstrumentationLibrary, _ pdata.Resource) interface{} {
	return l.value
}

func newGetter(val common.Value) (getter, error) {
	if s := val.String; s != nil {
		return &literal{
			value: *s,
		}, nil
	} else if f := val.Float; f != nil {
		return &literal{
			value: *f,
		}, nil
	} else if i := val.Int; i != nil {
		return &literal{
			value: *i,
		}, nil
	}

	if val.Path != nil {
		return newPathGetSetter(val.Path.Fields)
	}

	if val.Invocation == nil {
		// In practice, can't happen since the DSL grammar guarantees one is set
		return nil, fmt.Errorf("no value field set. This is a bug in the transformprocessor")
	}

	call, err := newFunctionCall(*val.Invocation)
	if err != nil {
		return nil, err
	}
	return &pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return call(span, il, resource)
		},
	}, nil
}
