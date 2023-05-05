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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func createRouteFunction[K any](_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (interface{}, error) {
		return true, nil
	}, nil
}

func Functions[K any]() map[string]ottl.Factory[K] {
	return ottl.CreateFactoryMap(
		ottlfuncs.NewIsMatchFactory[K](),
		ottlfuncs.NewDeleteKeyFactory[K](),
		ottlfuncs.NewDeleteMatchingKeysFactory[K](),
		// noop function, it is required since the parsing of conditions is not implemented yet,
		////github.com/open-telemetry/opentelemetry-collector-contrib/issues/13545
		ottl.NewFactory("route", nil, createRouteFunction[K]),
	)
}
