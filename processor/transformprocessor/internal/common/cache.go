// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// LoadContextCache retrieves a context cache for the given context ID.
// If `sharedCache` is true, it returns the cached context map if it exists,
// or returns nil if it does not.
func LoadContextCache(cache map[ContextID]*pcommon.Map, context ContextID, sharedCache bool) *pcommon.Map {
	if !sharedCache {
		return nil
	}
	return cache[context]
}
