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
	"strings"
	"unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func ConvertCase[K any](target ottl.Getter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx K) (interface{}, error) {
		val, err := target.Get(ctx)

		if err != nil {
			return nil, err
		}

		if val != nil {
			if valStr, ok := val.(string); ok {

				if valStr == "" {
					return valStr, nil
				}
				if len(valStr) == 1 {
					return strings.ToLower(valStr), nil
				}
				source := []rune(valStr)
				dist := strings.Builder{}
				dist.Grow(len(valStr) + len(valStr)/3)
				skipNext := false

				for i := 0; i < len(source); i++ {
					cur := source[i]

					switch cur {
					case '-', '_', '.':
						dist.WriteRune('.')
						skipNext = true
						continue
					}
					if unicode.IsLower(cur) || unicode.IsDigit(cur) {
						dist.WriteRune(cur)
						continue
					}

					if i == 0 {
						dist.WriteRune(unicode.ToLower(cur))
						continue
					}

					last := source[i-1]

					if (!unicode.IsLetter(last)) || unicode.IsLower(last) {
						if skipNext {
							skipNext = false
						} else {
							dist.WriteRune('.')
						}
						dist.WriteRune(unicode.ToLower(cur))
						continue
					}

					if i < len(source)-1 {
						next := source[i+1]
						if unicode.IsLower(next) {
							if skipNext {
								skipNext = false
							} else {
								dist.WriteRune('.')
							}
							dist.WriteRune(unicode.ToLower(cur))
							continue
						}
					}

					dist.WriteRune(unicode.ToLower(cur))
				}

				return dist.String(), nil
			}
		}
		return nil, nil
	}, nil
}
