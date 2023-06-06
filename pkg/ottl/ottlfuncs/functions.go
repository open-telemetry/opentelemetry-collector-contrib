// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func StandardFuncs[K any]() map[string]ottl.Factory[K] {
	f := []ottl.Factory[K]{
		// Editors
		NewDeleteKeyFactory[K](),
		NewDeleteMatchingKeysFactory[K](),
		NewKeepKeysFactory[K](),
		NewLimitFactory[K](),
		NewMergeMapsFactory[K](),
		NewReplaceAllMatchesFactory[K](),
		NewReplaceAllPatternsFactory[K](),
		NewReplaceMatchFactory[K](),
		NewReplacePatternFactory[K](),
		NewSetFactory[K](),
		NewTruncateAllFactory[K](),
	}
	f = append(f, converters[K]()...)

	return ottl.CreateFactoryMap(f...)
}

func StandardConverters[K any]() map[string]ottl.Factory[K] {
	return ottl.CreateFactoryMap(converters[K]()...)
}

func converters[K any]() []ottl.Factory[K] {
	return []ottl.Factory[K]{
		// Converters
		NewConcatFactory[K](),
		NewConvertCaseFactory[K](),
		NewFnvFactory[K](),
		NewIntFactory[K](),
		NewIsMatchFactory[K](),
		NewLogFactory[K](),
		NewParseJSONFactory[K](),
		NewSHA1Factory[K](),
		NewSHA256Factory[K](),
		NewSpanIDFactory[K](),
		NewSplitFactory[K](),
		NewSubstringFactory[K](),
		NewTraceIDFactory[K](),
		NewUUIDFactory[K](),
	}
}
