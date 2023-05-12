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

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type LogArguments[K any] struct {
	Target ottl.Getter[K] `ottlarg:"0"`
}

func NewLogFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Log", &LogArguments[K]{}, createLogFunction[K])
}

func createLogFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*LogArguments[K])

	if !ok {
		return nil, fmt.Errorf("LogFactory args must be of type *LogArguments[K]")
	}

	return logFunc(args.Target), nil
}

func logFunc[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch value := value.(type) {
		case int64:
			if value <= 0 {
				return nil, fmt.Errorf("expected value greater than zero but got nil %v", value)
			}
			return math.Log((float64)(value)), nil
		case string:
			fltValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return nil, err
			}
			if fltValue <= 0 {
				return nil, fmt.Errorf("expected value greater than zero but got nil %v", fltValue)
			}
			return math.Log(fltValue), nil
		case float64:
			if value <= 0 {
				return nil, fmt.Errorf("expected value greater than zero but got nil %v", value)
			}
			return math.Log(value), nil
		default:
			return nil, fmt.Errorf("unhanlded type %T", value)
		}
	}
}
