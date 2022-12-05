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
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func KeepKeys[K any](target ottl.Getter[K], keys []string) (ottl.ExprFunc[K], error) {
	keySet := newKeyMap(keys)
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		mapVal, err := getMapTarget(ctx, tCtx, target)
		if err != nil {
			return nil, err
		}
		keepKeys(mapVal, keySet)
		return nil, nil
	}, nil
}

func newKeyMap(keys []string) map[string]struct{} {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	return keySet
}

func getMapTarget[K any](ctx context.Context, tCtx K, target ottl.Getter[K]) (pcommon.Map, error) {
	val, err := target.Get(ctx, tCtx)
	if err != nil {
		return pcommon.Map{}, err
	}
	mapVal, ok := val.(pcommon.Map)
	if !ok {
		return pcommon.Map{}, fmt.Errorf("target must be a pcommon.map but got %T", val)
	}
	return mapVal, nil
}

func keepKeys(mapVal pcommon.Map, keySet map[string]struct{}) {
	mapVal.RemoveIf(func(key string, value pcommon.Value) bool {
		_, ok := keySet[key]
		return !ok
	})
	if mapVal.Len() == 0 {
		mapVal.Clear()
	}
}
