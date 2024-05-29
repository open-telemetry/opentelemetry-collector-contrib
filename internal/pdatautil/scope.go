// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// GroupByScopeLogs groups LogRecords by scope. Modifications are made in place.
func GroupByScopeLogs(sls plog.ScopeLogsSlice) {
	// Hash each ScopeLogs based on identifying information.
	scopeHashes := make([][16]byte, sls.Len())
	for i := 0; i < sls.Len(); i++ {
		scopeHashes[i] = HashScopeLogs(sls.At(i))
	}

	// Find the first occurrence of each hash and note the index.
	firstScopeIndex := make([]int, sls.Len())
	for i := 0; i < sls.Len(); i++ {
		firstScopeIndex[i] = i
		for j := 0; j < i; j++ {
			if scopeHashes[i] == scopeHashes[j] {
				firstScopeIndex[i] = j
				break
			}
		}
	}

	// Merge ScopeLogs with the same hash.
	for i := 0; i < sls.Len(); i++ {
		if i == firstScopeIndex[i] {
			// This is the first occurrence of this hash.
			continue
		}
		sls.At(i).LogRecords().MoveAndAppendTo(sls.At(firstScopeIndex[i]).LogRecords())
	}

	// Remove the ScopeLogs which were merged onto others.
	i := 0
	sls.RemoveIf(func(plog.ScopeLogs) bool {
		remove := i != firstScopeIndex[i]
		i++
		return remove
	})
}

// Creates a hash based on the ScopeLogs attributes, name, and version
func HashScopeLogs(sl plog.ScopeLogs) [16]byte {
	scopeHash := pcommon.NewMap()
	scopeHash.PutStr("schema_url", sl.SchemaUrl())
	scopeHash.PutStr("name", sl.Scope().Name())
	scopeHash.PutStr("version", sl.Scope().Version())
	scopeHash.PutStr("attributes_hash", hashToString(pdatautil.MapHash(sl.Scope().Attributes())))
	return pdatautil.MapHash(scopeHash)
}

func hashToString(hash [16]byte) string {
	return string(hash[:])
}
