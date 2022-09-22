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
	"fmt"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl"
)

func ReplaceAllMatches(target ottl.GetSetter, pattern string, replacement string) (ottl.ExprFunc, error) {
	glob, err := glob.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("the pattern supplied to replace_match is not a valid pattern: %w", err)
	}
	return func(ctx ottl.TransformContext) interface{} {
		val := target.Get(ctx)
		if val == nil {
			return nil
		}
		if attrs, ok := val.(pcommon.Map); ok {
			updated := pcommon.NewMap()
			attrs.CopyTo(updated)
			updated.Range(func(key string, value pcommon.Value) bool {
				if glob.Match(value.StringVal()) {
					value.SetStringVal(replacement)
				}
				return true
			})
			target.Set(ctx, updated)
		}
		return nil
	}, nil
}
