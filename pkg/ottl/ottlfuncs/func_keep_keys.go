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

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type KeepKeysArguments[K any] struct {
	Target ottl.PMapGetter[K] `ottlarg:"0"`
	Keys   []string           `ottlarg:"1"`
}

func NewKeepKeysFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("keep_keys", &KeepKeysArguments[K]{}, createKeepKeysFunction[K])
}

func createKeepKeysFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*KeepKeysArguments[K])

	if !ok {
		return nil, fmt.Errorf("KeepKeysFactory args must be of type *KeepKeysArguments[K]")
	}

	return keepKeys(args.Target, args.Keys), nil
}

func keepKeys[K any](target ottl.PMapGetter[K], keys []string) ottl.ExprFunc[K] {
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
	}
}
