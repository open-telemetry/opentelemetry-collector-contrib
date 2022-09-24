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

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Limit(target ottl.GetSetter, limit int64, priorityKeys []string) (ottl.ExprFunc, error) {
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit for limit function, %d cannot be negative", limit)
	}
	if limit < int64(len(priorityKeys)) {
		return nil, fmt.Errorf(
			"invalid limit for limit function, %d cannot be less than number of priority attributes %d",
			limit, len(priorityKeys),
		)
	}
	keep := make(map[string]struct{}, len(priorityKeys))
	for _, key := range priorityKeys {
		keep[key] = struct{}{}
	}

	return func(ctx ottl.TransformContext) interface{} {
		val := target.Get(ctx)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pcommon.Map); ok {
			if int64(attrs.Len()) <= limit {
				return nil
			}

			count := int64(0)
			for _, key := range priorityKeys {
				if _, ok := attrs.Get(key); ok {
					count++
				}
			}

			attrs.RemoveIf(func(key string, value pcommon.Value) bool {
				if _, ok := keep[key]; ok {
					return false
				}
				if count < limit {
					count++
					return false
				}
				return true
			})
			// TODO: Write log when limiting is performed
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9730
		}
		return nil
	}, nil
}
