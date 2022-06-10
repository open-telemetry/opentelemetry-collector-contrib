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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"fmt"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func replaceAllMatches(target GetSetter, pattern string, replacement string) (ExprFunc, error) {
	glob, err := glob.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("the pattern supplied to replace_match is not a valid pattern, %v", err)
	}
	return func(ctx TransformContext) interface{} {
		val := target.Get(ctx)
		if val == nil {
			return nil
		}
		if attrs, ok := val.(pcommon.Map); ok {
			updated := pcommon.NewMap()
			updated.EnsureCapacity(attrs.Len())
			attrs.Range(func(key string, value pcommon.Value) bool {
				if glob.Match(value.StringVal()) {
					updated.InsertString(key, replacement)
				} else {
					updated.Insert(key, value)
				}
				return true
			})
			target.Set(ctx, updated)
		}
		return nil
	}, nil
}
