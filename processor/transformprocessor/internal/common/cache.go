// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func NewContextCache(cache map[ContextID]*pcommon.Map, context ContextID, sharedCache bool) *pcommon.Map {
	if !sharedCache {
		m := pcommon.NewMap()
		return &m
	}
	existing, ok := cache[context]
	if ok {
		return existing
	}
	m := pcommon.NewMap()
	cache[context] = &m
	return &m
}
