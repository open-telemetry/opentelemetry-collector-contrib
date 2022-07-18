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

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"

// testGetSetter is a getSetter for testing.
type testGetSetter struct {
	getter tql.ExprFunc
	setter func(ctx tql.TransformContext, val interface{})
}

func (path testGetSetter) Get(ctx tql.TransformContext) interface{} {
	return path.getter(ctx)
}

func (path testGetSetter) Set(ctx tql.TransformContext, val interface{}) {
	path.setter(ctx, val)
}

type testLiteral struct {
	value interface{}
}

func (t testLiteral) Get(ctx tql.TransformContext) interface{} {
	return t.value
}
