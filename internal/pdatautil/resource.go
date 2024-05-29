// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pdatautil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"

import (
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

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
