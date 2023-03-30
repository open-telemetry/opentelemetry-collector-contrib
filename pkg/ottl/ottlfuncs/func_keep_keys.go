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
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func KeepKeys[K any](target ottl.PMapGetter[K], keys []string) (ottl.ExprFunc[K], error) {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}

	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val.RemoveIf(func(key string, value pcommon.Value) bool {
			_, ok := keySet[key]
			return !ok
		})
		if val.Len() == 0 {
			val.Clear()
		}
		return nil, nil
	}, nil
}
