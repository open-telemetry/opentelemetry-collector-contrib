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

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ParseJSONArguments[K any] struct {
	Target ottl.StringGetter[K] `ottlarg:"0"`
}

func NewParseJSONFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("ParseJSON", &ParseJSONArguments[K]{}, createParseJSONFunction[K])
}

func createParseJSONFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ParseJSONArguments[K])

	if !ok {
		return nil, fmt.Errorf("ParseJSONFactory args must be of type *ParseJSONArguments[K]")
	}

	return parseJSON(args.Target), nil
}

// parseJSON returns a `pcommon.Map` struct that is a result of parsing the target string as JSON
// Each JSON type is converted into a `pdata.Value` using the following map:
//
//	JSON boolean -> bool
//	JSON number  -> float64
//	JSON string  -> string
//	JSON null    -> nil
//	JSON arrays  -> pdata.SliceValue
//	JSON objects -> map[string]any
func parseJSON[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		var parsedValue map[string]interface{}
		err = jsoniter.UnmarshalFromString(targetVal, &parsedValue)
		if err != nil {
			return nil, err
		}
		result := pcommon.NewMap()
		err = result.FromRaw(parsedValue)
		return result, err
	}
}
