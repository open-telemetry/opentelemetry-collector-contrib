// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// LoadContextCache retrieves or creates a context cache for the given context ID.
// If `sharedCache` is true, it returns the cached context map if it exists,
// or creates and stores a new one if it does not. If `sharedCache` is false, it returns nil.
func LoadContextCache(cache map[ContextID]*pcommon.Map, context ContextID, sharedCache bool) *pcommon.Map {
	if !sharedCache {
		return nil
	}
	v, ok := cache[context]
	if ok {
		return v
	}
	m := pcommon.NewMap()
	cache[context] = &m
	return &m
}
