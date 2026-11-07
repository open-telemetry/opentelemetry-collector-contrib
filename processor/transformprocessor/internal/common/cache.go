// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/transformconfig"
)

// LoadContextCache retrieves a context cache for the given context ID.
// If `sharedCache` is true, it returns the cached context map if it exists,
// or returns nil if it does not.
func LoadContextCache(cache map[transformconfig.ContextID]*pcommon.Map, context transformconfig.ContextID, sharedCache bool) *pcommon.Map {
	if !sharedCache || len(cache) == 0 {
		return nil
	}
	return cache[context]
}

// NewSharedCaches builds a fresh set of shared cache maps for a single
// processing invocation, with one map per context ID in contexts. It returns
// nil when contexts is empty, which LoadContextCache treats as "no shared
// cache configured".
func NewSharedCaches(contexts []transformconfig.ContextID) map[transformconfig.ContextID]*pcommon.Map {
	if len(contexts) == 0 {
		return nil
	}
	caches := make(map[transformconfig.ContextID]*pcommon.Map, len(contexts))
	for _, id := range contexts {
		m := pcommon.NewMap()
		caches[id] = &m
	}
	return caches
}
