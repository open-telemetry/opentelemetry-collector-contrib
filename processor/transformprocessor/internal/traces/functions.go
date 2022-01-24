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
	"go.opentelemetry.io/collector/model/pdata"
)

func set(target getSetter, value getter) exprFunc {
	return func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
		val := value.get(span, il, resource)
		if val != nil {
			target.set(span, il, resource, val)
		}
		return nil
	}
}

func keep(target getSetter, keys []string) exprFunc {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}

	return func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
		val := target.get(span, il, resource)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pdata.AttributeMap); ok {
			filtered := pdata.NewAttributeMap()
			attrs.Range(func(key string, val pdata.AttributeValue) bool {
				if _, ok := keySet[key]; ok {
					filtered.Insert(key, val)
				}
				return true
			})
			target.set(span, il, resource, filtered)
		}
		return nil
	}
}

func init() {
	registerFunction("keep", keep)
	registerFunction("set", set)
}
