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

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func truncateAll(target GetSetter, limit int64) (ExprFunc, error) {
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit for truncate_all function, %d cannot be negative", limit)
	}
	return func(ctx TransformContext) interface{} {
		if limit < 0 {
			return nil
		}

		val := target.Get(ctx)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pcommon.Map); ok {
			updated := pcommon.NewMap()
			updated.EnsureCapacity(attrs.Len())
			attrs.Range(func(key string, val pcommon.Value) bool {
				stringVal := val.StringVal()
				if int64(len(stringVal)) > limit {
					updated.InsertString(key, stringVal[:limit])
				} else {
					updated.Insert(key, val)
				}
				return true
			})
			target.Set(ctx, updated)
			// TODO: Write log when truncation is performed
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9730
		}
		return nil
	}, nil
}
