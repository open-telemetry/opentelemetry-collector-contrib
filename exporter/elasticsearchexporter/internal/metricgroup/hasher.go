// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricgroup // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metricgroup"

import (
	"encoding/binary"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// HashKey is a struct for comparing data point identity.
type HashKey struct {
	resourceHash uint64
	scopeHash    uint64
	dpHash       uint64
}

// DataPointHasher is an interface for hashing data points by their identity,
// for grouping into a single document.
type DataPointHasher interface {
	UpdateResource(pcommon.Resource)
	UpdateScope(pcommon.InstrumentationScope)
	UpdateDataPoint(datapoints.DataPoint)
	HashKey() HashKey
}

type hashableDataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Metric() pmetric.Metric
	Attributes() pcommon.Map
}

// ECSDataPointHasher caches sorted resource attributes and the current data point.
// HashKey merges datapoint attrs over resource attrs (ECS root-level overwrite) without
// allocating a merged pcommon.Map.
type ECSDataPointHasher struct {
	resourceKVs []kv
	dpKVs       []kv
	hasher      xxhash.Digest
	dp          hashableDataPoint
}

func (h *ECSDataPointHasher) UpdateResource(resource pcommon.Resource) {
	h.resourceKVs = sortedKVsExcludeReservedAttrs(resource.Attributes(), elasticsearch.MappingHintsAttrKey)
}

func (*ECSDataPointHasher) UpdateScope(pcommon.InstrumentationScope) {
}

func (h *ECSDataPointHasher) UpdateDataPoint(dp datapoints.DataPoint) {
	h.dp = dp
}

func (h *ECSDataPointHasher) HashKey() HashKey {
	h.hasher.Reset()

	var timestampBuf [8]byte
	binary.LittleEndian.PutUint64(timestampBuf[:], uint64(h.dp.Timestamp()))
	_, _ = h.hasher.Write(timestampBuf[:])

	// scope attributes are ignored in ECS mode; datapoint attrs overwrite resource attrs
	dpAttrs := h.dp.Attributes()
	h.dpKVs = appendSortedKVsExcludeReservedAttrs(resetKVs(h.dpKVs, dpAttrs.Len()), dpAttrs, elasticsearch.MappingHintsAttrKey)
	writeMergedSortedKVs(&h.hasher, h.resourceKVs, h.dpKVs)

	hashKey := HashKey{dpHash: h.hasher.Sum64()}
	h.dpKVs = resetKVs(h.dpKVs, 0)
	return hashKey
}

// OTelDataPointHasher computes a hash for each of resource, scope and data point on each Update call,
// to avoid wasteful hashing and sorting on data point sharing the same resource and scope.
type OTelDataPointHasher struct {
	resourceHash uint64
	scopeHash    uint64
	dpHash       uint64
	kvs          []kv
	hasher       xxhash.Digest
}

func (h *OTelDataPointHasher) UpdateResource(resource pcommon.Resource) {
	// We cannot use exp/metrics/identity here because some resource fields e.g. schema url
	// are not dimensions and should not be part of the hash.

	h.hasher.Reset()
	// There is special handling to merge geo attributes during serialization,
	// but we can hash them as if they are separate now.
	h.kvs = mapHashSortedExcludeReservedAttrs(&h.hasher, h.kvs, resource.Attributes(), elasticsearch.MappingHintsAttrKey)
	h.resourceHash = h.hasher.Sum64()
	h.kvs = resetKVs(h.kvs, 0)
}

func (h *OTelDataPointHasher) UpdateScope(scope pcommon.InstrumentationScope) {
	h.hasher.Reset()
	_, _ = h.hasher.WriteString(scope.Name())
	// There is special handling to merge geo attributes during serialization,
	// but we can hash them as if they are separate now.
	h.kvs = mapHashSortedExcludeReservedAttrs(&h.hasher, h.kvs, scope.Attributes(), elasticsearch.MappingHintsAttrKey)
	h.scopeHash = h.hasher.Sum64()
	h.kvs = resetKVs(h.kvs, 0)
}

func (h *OTelDataPointHasher) UpdateDataPoint(dp datapoints.DataPoint) {
	h.hasher.Reset()

	var timestampBuf [8]byte
	binary.LittleEndian.PutUint64(timestampBuf[:], uint64(dp.Timestamp()))
	_, _ = h.hasher.Write(timestampBuf[:])

	binary.LittleEndian.PutUint64(timestampBuf[:], uint64(dp.StartTimestamp()))
	_, _ = h.hasher.Write(timestampBuf[:])

	_, _ = h.hasher.WriteString(dp.Metric().Unit())

	// There is special handling to merge geo attributes during serialization,
	// but we can hash them as if they are separate now.
	h.kvs = mapHashSortedExcludeReservedAttrs(&h.hasher, h.kvs, dp.Attributes(), elasticsearch.MappingHintsAttrKey)

	h.dpHash = h.hasher.Sum64()
	h.kvs = resetKVs(h.kvs, 0)
}

func (h *OTelDataPointHasher) HashKey() HashKey {
	return HashKey{
		resourceHash: h.resourceHash,
		scopeHash:    h.scopeHash,
		dpHash:       h.dpHash,
	}
}
