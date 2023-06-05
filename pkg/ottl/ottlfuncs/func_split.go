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
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type SplitArguments[K any] struct {
	Target    ottl.StringGetter[K] `ottlarg:"0"`
	Delimiter string               `ottlarg:"1"`
}

func NewSplitFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Split", &SplitArguments[K]{}, createSplitFunction[K])
}

func createSplitFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*SplitArguments[K])

	if !ok {
		return nil, fmt.Errorf("SplitFactory args must be of type *SplitArguments[K]")
	}

	return split(args.Target, args.Delimiter), nil
}

func split[K any](target ottl.StringGetter[K], delimiter string) ottl.ExprFunc[K] {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return strings.Split(val, delimiter), nil
	}
}
