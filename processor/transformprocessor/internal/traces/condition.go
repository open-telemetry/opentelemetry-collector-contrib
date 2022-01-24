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

type condFunc = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) bool

func newConditionEvaluator(cond *common.Condition) (condFunc, error) {
	if cond == nil {
		return func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) bool {
			return true
		}, nil
	}
	left, err := newGetter(cond.Left)
	if err != nil {
		return nil, err
	}
	right, err := newGetter(cond.Right)
	if err != nil {
		return nil, err
	}

	switch cond.Op {
	case "==":
		return func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) bool {
			a := left.get(span, il, resource)
			b := right.get(span, il, resource)
			return a == b
		}, nil
	case "!=":
		return func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) bool {
			return left.get(span, il, resource) != right.get(span, il, resource)
		}, nil
	}

	return nil, fmt.Errorf("unrecognized boolean operation %v", cond.Op)
}
