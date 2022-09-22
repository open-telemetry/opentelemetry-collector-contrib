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
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Concat(delimiter string, vals []ottl.Getter) (ottl.ExprFunc, error) {
	return func(ctx ottl.TransformContext) interface{} {
		builder := strings.Builder{}
		for i, rv := range vals {
			switch val := rv.Get(ctx).(type) {
			case string:
				builder.WriteString(val)
			case []byte:
				builder.WriteString(fmt.Sprintf("%x", val))
			case int64:
				builder.WriteString(fmt.Sprint(val))
			case float64:
				builder.WriteString(fmt.Sprint(val))
			case bool:
				builder.WriteString(fmt.Sprint(val))
			case nil:
				builder.WriteString(fmt.Sprint(val))
			}

			if i != len(vals)-1 {
				builder.WriteString(delimiter)
			}
		}
		return builder.String()
	}, nil
}
