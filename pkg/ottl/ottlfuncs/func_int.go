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
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type IntArguments[K any] struct {
	Target ottl.Getter[K] `ottlarg:"0"`
}

func NewIntFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Int", &IntArguments[K]{}, createIntFunction[K])
}

func createIntFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*IntArguments[K])

	if !ok {
		return nil, fmt.Errorf("IntFactory args must be of type *IntArguments[K]")
	}

	return intFunc(args.Target), nil
}

func intFunc[K any](target ottl.Getter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		value, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		switch value := value.(type) {
		case int64:
			return value, nil
		case string:
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, nil
			}

			return intValue, nil
		case float64:
			return (int64)(value), nil
		case bool:
			if value {
				return int64(1), nil
			}
			return int64(0), nil
		default:
			return nil, nil
		}
	}
}
