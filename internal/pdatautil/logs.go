// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// FlattenResourceLogs moves each LogRecord onto a dedicated ResourceLogs and ScopeLogs.
// Modifications are made in place. Order of LogRecords is preserved.
func FlattenLogs(rls plog.ResourceLogsSlice) {
	tmp := plog.NewResourceLogsSlice()
	rls.MoveAndAppendTo(tmp)
	for i := 0; i < tmp.Len(); i++ {
		groupedResource := tmp.At(i)
		for j := 0; j < groupedResource.ScopeLogs().Len(); j++ {
			groupedScope := groupedResource.ScopeLogs().At(j)
			for k := 0; k < groupedScope.LogRecords().Len(); k++ {
				flatResource := rls.AppendEmpty()
				groupedResource.Resource().Attributes().CopyTo(flatResource.Resource().Attributes())
				flatScope := flatResource.ScopeLogs().AppendEmpty()
				flatScope.SetSchemaUrl(groupedScope.SchemaUrl())
				flatScope.Scope().SetName(groupedScope.Scope().Name())
				flatScope.Scope().SetVersion(groupedScope.Scope().Version())
				groupedScope.Scope().Attributes().CopyTo(flatScope.Scope().Attributes())
				groupedScope.LogRecords().At(k).CopyTo(flatScope.LogRecords().AppendEmpty())
			}
		}
	}
}

// GroupByResourceLogs groups ScopeLogs by Resource. Modifications are made in place.
func GroupByResourceLogs(rls plog.ResourceLogsSlice) {
	// Hash each ResourceLogs based on identifying information.
	resourceHashes := make([][16]byte, rls.Len())
	for i := 0; i < rls.Len(); i++ {
		resourceHashes[i] = pdatautil.MapHash(rls.At(i).Resource().Attributes())
	}

	// Find the first occurrence of each hash and note the index.
	firstScopeIndex := make([]int, rls.Len())
	for i := 0; i < rls.Len(); i++ {
		firstScopeIndex[i] = i
		for j := 0; j < i; j++ {
			if resourceHashes[i] == resourceHashes[j] {
				firstScopeIndex[i] = j
				break
			}
		}
	}

	// Merge Resources with the same hash.
	for i := 0; i < rls.Len(); i++ {
		if i == firstScopeIndex[i] {
			// This is the first occurrence of this hash.
			continue
		}
		rls.At(i).ScopeLogs().MoveAndAppendTo(rls.At(firstScopeIndex[i]).ScopeLogs())
	}

	// Remove the ResourceLogs which were merged onto others.
	i := 0
	rls.RemoveIf(func(plog.ResourceLogs) bool {
		remove := i != firstScopeIndex[i]
		i++
		return remove
	})

	// Merge ScopeLogs within each ResourceLogs.
	for i := 0; i < rls.Len(); i++ {
		GroupByScopeLogs(rls.At(i).ScopeLogs())
	}
}

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
	attrHash := pdatautil.MapHash(sl.Scope().Attributes())
	scopeHash.PutStr("attributes_hash", string(attrHash[:]))
	return pdatautil.MapHash(scopeHash)
}
