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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func Functions[K any]() map[string]ottl.Factory[K] {
	return ottl.CreateFactoryMap(
		ottlfuncs.NewTraceIDFactory[K](),
		ottlfuncs.NewSpanIDFactory[K](),
		ottlfuncs.NewIsMatchFactory[K](),
		ottlfuncs.NewConcatFactory[K](),
		ottlfuncs.NewSplitFactory[K](),
		ottlfuncs.NewIntFactory[K](),
		ottlfuncs.NewConvertCaseFactory[K](),
		ottlfuncs.NewParseJSONFactory[K](),
		ottlfuncs.NewSubstringFactory[K](),
		ottlfuncs.NewKeepKeysFactory[K](),
		ottlfuncs.NewSetFactory[K](),
		ottlfuncs.NewTruncateAllFactory[K](),
		ottlfuncs.NewLimitFactory[K](),
		ottlfuncs.NewReplaceMatchFactory[K](),
		ottlfuncs.NewReplaceAllMatchesFactory[K](),
		ottlfuncs.NewReplacePatternFactory[K](),
		ottlfuncs.NewReplaceAllPatternsFactory[K](),
		ottlfuncs.NewDeleteKeyFactory[K](),
		ottlfuncs.NewDeleteMatchingKeysFactory[K](),
		ottlfuncs.NewMergeMapsFactory[K](),
	)
}

func ResourceFunctions() map[string]ottl.Factory[ottlresource.TransformContext] {
	return Functions[ottlresource.TransformContext]()
}

func ScopeFunctions() map[string]ottl.Factory[ottlscope.TransformContext] {
	return Functions[ottlscope.TransformContext]()
}
