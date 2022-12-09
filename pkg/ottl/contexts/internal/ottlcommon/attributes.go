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

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"

import "errors"

func CheckValuesHomogeneous(val interface{}) error {
	if items, ok := val.([]interface{}); ok && len(items) > 0 {
		switch items[0].(type) {
		case string:
			return assertAllValues[string](items)
		case bool:
			return assertAllValues[bool](items)
		case int64:
			return assertAllValues[int64](items)
		case float64:
			return assertAllValues[float64](items)
		case []byte:
			return assertAllValues[[]byte](items)
		default:
			return errors.New("unsupported type")
		}
	}
	return nil
}

func assertAllValues[V int64 | float64 | string | bool | []byte](vals []interface{}) error {
	for _, val := range vals {
		if _, ok := val.(V); !ok {
			return errors.New("arrays in attribute values are required to be homogeneou")
		}
	}
	return nil
}
