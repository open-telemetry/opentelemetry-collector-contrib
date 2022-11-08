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
	"strings"

	"github.com/iancoleman/strcase"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func ConvertCase[K any](target ottl.Getter[K], toCase string) (ottl.ExprFunc[K], error) {
	if toCase != "lower" && toCase != "upper" && toCase != "snake" && toCase != "camel" {
		return nil, fmt.Errorf("invalid case: %s, allowed cases are: lower, upper, snake, camel", toCase)
	}

	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)

		if err != nil {
			return nil, err
		}

		if val != nil {

			if valStr, ok := val.(string); ok {

				if valStr == "" {
					return valStr, nil
				}

				switch toCase {
				// Convert string to lowercase (SOME_NAME -> some_name)
				case "lower":
					return convertCaseLower(valStr), nil

				// Convert string to uppercase (some_name -> SOME_NAME)
				case "upper":
					return convertCaseUpper(valStr), nil

				// Convert string to snake case (someName -> some_name)
				case "snake":
					return convertCaseSnake(valStr), nil

				// Convert string to camel case (some_name -> SomeName)
				case "camel":
					return convertCaseCamel(valStr), nil

				default:
					return nil, fmt.Errorf("error handling unexpected case: %s", toCase)
				}
			}
		}
		return nil, nil
	}, nil
}

func convertCaseLower(input string) string {
	return strings.ToLower(input)
}

func convertCaseUpper(input string) string {
	return strings.ToUpper(input)
}

func convertCaseSnake(input string) string {
	return strcase.ToSnake(input)
}

func convertCaseCamel(input string) string {
	return strcase.ToCamel(input)
}
