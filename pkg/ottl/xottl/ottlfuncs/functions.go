// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package ottlfuncs houses OTTL functions that are excluded from OTTL's
// stability guarantees. When the pkg.ottl.functions.enableExperimental feature
// gate is enabled they are included in the stable ottlfuncs.StandardFuncs and
// ottlfuncs.StandardConverters.
package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

// ExperimentalFuncs returns all experimental functions (editors and converters) in this package.
func ExperimentalFuncs[K any]() []ottl.Factory[K] {
	return append(editors[K](), ExperimentalConverters[K]()...)
}

// ExperimentalConverters returns the converters that are excluded from OTTL's
// stability guarantees. Components merge these into their function set to make
// them available in OTTL statements.
func ExperimentalConverters[K any]() []ottl.Factory[K] {
	return []ottl.Factory[K]{
		NewAllFactory[K](),
		NewAnyFactory[K](),
		NewFilterFactory[K](),
		NewFindFactory[K](),
		NewMapEachFactory[K](),
		NewMapKeysFactory[K](),
		NewReduceFactory[K](),
		NewWhenFactory[K](),
	}
}

// WithExperimentalConverters adds the experimental converters to the given function map and returns it.
func WithExperimentalConverters[K any](funcs map[string]ottl.Factory[K]) map[string]ottl.Factory[K] {
	for _, f := range ExperimentalConverters[K]() {
		funcs[f.Name()] = f
	}
	return funcs
}

func editors[K any]() []ottl.Factory[K] {
	return []ottl.Factory[K]{}
}
