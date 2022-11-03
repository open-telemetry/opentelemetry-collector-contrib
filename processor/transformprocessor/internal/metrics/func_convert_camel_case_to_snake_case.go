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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"strings"
	"unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
)

func convertCamelCaseToSnakeCase() (ottl.ExprFunc[ottldatapoints.TransformContext], error) {
	return func(ctx ottldatapoints.TransformContext) (interface{}, error) {
		metric := ctx.GetMetric()

		s := metric.Name()

		if s == "" {
			return nil, nil
		}
		if len(s) == 1 {
			metric.SetName(strings.ToLower(s))
			return nil, nil
		}
		source := []rune(s)
		dist := strings.Builder{}
		dist.Grow(len(s) + len(s)/3)
		skipNext := false

		for i := 0; i < len(source); i++ {
			cur := source[i]

			switch cur {
			case '-', '_':
				dist.WriteRune('_')
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
					dist.WriteRune('_')
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
						dist.WriteRune('_')
					}
					dist.WriteRune(unicode.ToLower(cur))
					continue
				}
			}

			dist.WriteRune(unicode.ToLower(cur))
		}

		metric.SetName(dist.String())

		return nil, nil
	}, nil
}
