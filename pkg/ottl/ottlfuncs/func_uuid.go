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

	guuid "github.com/google/uuid"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func uuid[K any]() (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		u := guuid.New()
		return u.String(), nil
	}, nil
}

func createUUIDFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	return uuid[K]()
}

func NewUUIDFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("UUID", nil, createUUIDFunction[K])
}
