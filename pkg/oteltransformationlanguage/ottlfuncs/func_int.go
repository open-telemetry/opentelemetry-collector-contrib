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

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottlfuncs"

import (
	"strconv"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl"
)

func Int(target ottl.Getter) (ottl.ExprFunc, error) {
	return func(ctx ottl.TransformContext) interface{} {
		value := target.Get(ctx)
		switch value := value.(type) {
		case int64:
			return value
		case string:
			intValue, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil
			}

			return intValue
		case float64:
			return (int64)(value)
		case bool:
			if value {
				return int64(1)
			}
			return int64(0)
		default:
			return nil
		}
	}, nil
}
