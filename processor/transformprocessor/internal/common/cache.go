// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// LoadContextCache retrieves or creates a context cache map for the given context ID.
// If the cache is not found, a new map is created and stored in the contextCache map.
func LoadContextCache(contextCache map[ContextID]*pcommon.Map, context ContextID) *pcommon.Map {
	v, ok := contextCache[context]
	if ok {
		return v
	}
	m := pcommon.NewMap()
	contextCache[context] = &m
	return &m
}
