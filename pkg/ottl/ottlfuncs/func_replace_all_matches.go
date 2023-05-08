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

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ReplaceAllMatchesArguments[K any] struct {
	Target      ottl.PMapGetter[K] `ottlarg:"0"`
	Pattern     string             `ottlarg:"1"`
	Replacement string             `ottlarg:"2"`
}

func NewReplaceAllMatchesFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("replace_all_matches", &ReplaceAllMatchesArguments[K]{}, createReplaceAllMatchesFunction[K])
}

func createReplaceAllMatchesFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ReplaceAllMatchesArguments[K])

	if !ok {
		return nil, fmt.Errorf("ReplaceAllMatchesFactory args must be of type *ReplaceAllMatchesArguments[K]")
	}

	return replaceAllMatches(args.Target, args.Pattern, args.Replacement)
}

func replaceAllMatches[K any](target ottl.PMapGetter[K], pattern string, replacement string) (ottl.ExprFunc[K], error) {
	glob, err := glob.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("the pattern supplied to replace_match is not a valid pattern: %w", err)
	}
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		val.Range(func(key string, value pcommon.Value) bool {
			if glob.Match(value.Str()) {
				value.SetStr(replacement)
			}
			return true
		})
		return nil, nil
	}, nil
}
