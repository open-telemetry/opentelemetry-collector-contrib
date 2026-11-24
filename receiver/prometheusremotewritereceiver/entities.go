// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"slices"

	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

type cachedEntityEntry struct {
	info       prometheus.EntityInfo
	seriesHash uint64
	timestamp  int64
}

// cachedResourceMetrics stores a detached snapshot of resource attributes
// alongside cached otel_entity_info entries for a (job, instance) pair.
type cachedResourceMetrics struct {
	pmetric.ResourceMetrics
	entities      []cachedEntityEntry
	hasTargetInfo bool
}

func hashJobAndInstance(job, instance string) uint64 {
	h := identity.Resource{}.Hash()
	h.Write([]byte(job))
	h.Write(sep)
	h.Write([]byte(instance))
	return h.Sum64()
}

func isEntityInfoMetricType(ts *writev2.TimeSeries) bool {
	switch ts.Metadata.Type {
	case writev2.Metadata_METRIC_TYPE_INFO, writev2.Metadata_METRIC_TYPE_GAUGE:
		return true
	case writev2.Metadata_METRIC_TYPE_UNSPECIFIED:
		return len(ts.Histograms) == 0
	default:
		return false
	}
}

func latestSample(samples []writev2.Sample) writev2.Sample {
	latest := samples[0]
	for _, s := range samples[1:] {
		if s.Timestamp >= latest.Timestamp {
			latest = s
		}
	}
	return latest
}

func parseEntityInfoFromLabels(ls labels.Labels) (prometheus.EntityInfo, bool) {
	var entityType, schemaURL, rawIDKeys, rawDescKeys string
	ls.Range(func(l labels.Label) {
		switch l.Name {
		case prometheus.EntityTypeLabelKey:
			entityType = l.Value
		case prometheus.EntitySchemaURLLabelKey:
			schemaURL = l.Value
		case prometheus.EntityIDKeysLabelKey:
			rawIDKeys = l.Value
		case prometheus.EntityDescriptionKeysLabelKey:
			rawDescKeys = l.Value
		}
	})
	return prometheus.ParseEntityInfo(entityType, schemaURL, rawIDKeys, rawDescKeys)
}

func hasAllIDKeys(info prometheus.EntityInfo, attrs pcommon.Map) bool {
	if len(info.IDKeys) == 0 || attrs == (pcommon.Map{}) {
		return false
	}
	for _, k := range info.IDKeys {
		if _, ok := attrs.Get(k); !ok {
			return false
		}
	}
	return true
}

func sameEntityIdentity(a, b prometheus.EntityInfo) bool {
	if a.SchemaURL != b.SchemaURL || len(a.IDKeys) != len(b.IDKeys) {
		return false
	}
	aIDs := slices.Clone(a.IDKeys)
	bIDs := slices.Clone(b.IDKeys)
	slices.Sort(aIDs)
	slices.Sort(bIDs)
	return slices.Equal(aIDs, bIDs)
}

// mergeEntityEntries reconciles cached entities with active and stale
// otel_entity_info series from the current request. Valid active current-request
// entries replace cached entries with the same otel_entity_type, stale markers
// evict matching series with timestamp <= staleTS, and superseded non-mergeable
// same-type entries are pruned so later stale markers cannot resurrect them.
func mergeEntityEntries(cached, reqActive []cachedEntityEntry, reqStale map[uint64]int64, attrs pcommon.Map, hasTargetInfo bool) (merged []cachedEntityEntry, changed bool) {
	if len(reqActive) == 0 && len(reqStale) == 0 {
		return cached, false
	}
	// Filter and deduplicate active entries from the current request against
	// any StaleNaN marker in the same request.
	activeBySeries := make(map[uint64]int, len(reqActive))
	activeReq := make([]cachedEntityEntry, 0, len(reqActive))
	for _, entry := range reqActive {
		if staleTS, isStale := reqStale[entry.seriesHash]; isStale && entry.timestamp <= staleTS {
			continue
		}
		if idx, exists := activeBySeries[entry.seriesHash]; exists {
			if entry.timestamp >= activeReq[idx].timestamp {
				activeReq[idx] = entry
			}
			continue
		}
		activeBySeries[entry.seriesHash] = len(activeReq)
		activeReq = append(activeReq, entry)
	}

	// Only valid incoming entities may replace cached entities or enter the cache
	// once resource attributes are established. When target_info has not arrived
	// yet, keep incoming entities optimistically unless a non-evicted cached or
	// same-request valid entity of the same type already matches attrs.
	validTypes := make(map[string]bool, len(cached)+len(activeReq))
	for _, entry := range cached {
		if staleTS, isStale := reqStale[entry.seriesHash]; isStale && entry.timestamp <= staleTS {
			continue
		}
		if hasAllIDKeys(entry.info, attrs) {
			validTypes[entry.info.Type] = true
		}
	}
	for _, entry := range activeReq {
		if hasAllIDKeys(entry.info, attrs) {
			validTypes[entry.info.Type] = true
		}
	}
	validActiveReq := make([]cachedEntityEntry, 0, len(activeReq))
	for _, entry := range activeReq {
		if hasAllIDKeys(entry.info, attrs) {
			validActiveReq = append(validActiveReq, entry)
			continue
		}
		if hasTargetInfo || validTypes[entry.info.Type] {
			continue
		}
		validActiveReq = append(validActiveReq, entry)
	}

	replacedTypes := make(map[string]struct{}, len(validActiveReq))
	for _, entry := range validActiveReq {
		replacedTypes[entry.info.Type] = struct{}{}
	}

	combined := make([]cachedEntityEntry, 0, len(cached)+len(validActiveReq))
	var evicted bool
	for _, entry := range cached {
		if _, replaced := replacedTypes[entry.info.Type]; replaced {
			continue
		}
		if staleTS, isStale := reqStale[entry.seriesHash]; isStale && entry.timestamp <= staleTS {
			evicted = true
			continue
		}
		combined = append(combined, entry)
	}
	combined = append(combined, validActiveReq...)

	slices.SortStableFunc(combined, func(a, b cachedEntityEntry) int {
		if a.timestamp < b.timestamp {
			return -1
		}
		if a.timestamp > b.timestamp {
			return 1
		}
		return 0
	})

	// Walk in timestamp order and prune any earlier same-type entries superseded
	// by a later entry with a different schema_url or id_keys set, while keeping
	// mergeable same-identity entries for per-series StaleNaN eviction.
	activeByType := make(map[string][]cachedEntityEntry, len(combined))
	typeOrder := make([]string, 0, len(combined))
	for _, entry := range combined {
		existing, seen := activeByType[entry.info.Type]
		if !seen {
			typeOrder = append(typeOrder, entry.info.Type)
		}
		if len(existing) == 0 || !sameEntityIdentity(existing[0].info, entry.info) {
			activeByType[entry.info.Type] = []cachedEntityEntry{entry}
		} else {
			activeByType[entry.info.Type] = append(existing, entry)
		}
	}

	merged = make([]cachedEntityEntry, 0, len(combined))
	for _, entityType := range typeOrder {
		merged = append(merged, activeByType[entityType]...)
	}
	slices.SortStableFunc(merged, func(a, b cachedEntityEntry) int {
		if a.timestamp < b.timestamp {
			return -1
		}
		if a.timestamp > b.timestamp {
			return 1
		}
		return 0
	})

	return merged, len(validActiveReq) > 0 || evicted
}
