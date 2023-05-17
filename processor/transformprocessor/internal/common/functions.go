// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
		ottlfuncs.NewLogFactory[K](),
	)
}

func ResourceFunctions() map[string]ottl.Factory[ottlresource.TransformContext] {
	return Functions[ottlresource.TransformContext]()
}

func ScopeFunctions() map[string]ottl.Factory[ottlscope.TransformContext] {
	return Functions[ottlscope.TransformContext]()
}
