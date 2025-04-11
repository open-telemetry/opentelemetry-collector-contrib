// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"cmp"
	"encoding/binary"
	"hash"
	"hash/fnv"
	"math"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

type HashKey struct {
	resourceHash uint32
	scopeHash    uint32
	dpHash       uint32
}

// dataPointHasher is an interface for hashing data points by their identity,
// for grouping into a single document.
type dataPointHasher interface {
	UpdateResource(pcommon.Resource)
	UpdateScope(pcommon.InstrumentationScope)
	UpdateDataPoint(datapoints.DataPoint)
	HashKey() HashKey
}

func newDataPointHasher(mode MappingMode) dataPointHasher {
	switch mode {
	case MappingOTel:
		return &otelDataPointHasher{}
	default:
		// Defaults to ECS for backward compatibility
		return &ecsDataPointHasher{}
	}
}

// TODO use https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/internal/exp/metrics/identity

type hashableDataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Metric() pmetric.Metric
	Attributes() pcommon.Map
}

type (
	// ecsDataPointHasher solely relies on hashCombined because data point attributes overwrite resource attributes on merge.
	ecsDataPointHasher struct {
		resource pcommon.Resource
		dp       hashableDataPoint
	}
	// otelDataPointHasher does not use hashCombined as resource, scope and data points are independent.
	otelDataPointHasher struct {
		resourceHash uint32
		scopeHash    uint32
		dpHash       uint32
	}
)

func (h *ecsDataPointHasher) UpdateResource(resource pcommon.Resource) {
	h.resource = resource
}

func (h *ecsDataPointHasher) UpdateScope(_ pcommon.InstrumentationScope) {
}

func (h *ecsDataPointHasher) UpdateDataPoint(dp datapoints.DataPoint) {
	h.dp = dp
}

func (h *ecsDataPointHasher) HashKey() HashKey {
	merged := pcommon.NewMap()
	merged.EnsureCapacity(h.resource.Attributes().Len() + h.dp.Attributes().Len())
	h.resource.Attributes().CopyTo(merged)
	// scope attributes are ignored in ECS mode
	for k, v := range h.dp.Attributes().All() {
		v.CopyTo(merged.PutEmpty(k))
	}

	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(h.dp.Timestamp()))
	hasher.Write(timestampBuf)

	mapHashSortedExcludeReservedAttrs(hasher, merged)

	return HashKey{
		dpHash: hasher.Sum32(),
	}
}

func (h *otelDataPointHasher) UpdateResource(resource pcommon.Resource) {
	hasher := fnv.New32a()
	// TODO: handle geo attribute consistently as encoding logic
	mapHashSortedExcludeReservedAttrs(hasher, resource.Attributes(), elasticsearch.MappingHintsAttrKey)
	h.resourceHash = hasher.Sum32()
}

func (h *otelDataPointHasher) UpdateScope(scope pcommon.InstrumentationScope) {
	hasher := fnv.New32a()
	hasher.Write([]byte(scope.Name()))
	// TODO: handle geo attribute consistently as encoding logic
	mapHashSortedExcludeReservedAttrs(hasher, scope.Attributes(), elasticsearch.MappingHintsAttrKey)
	h.scopeHash = hasher.Sum32()
}

func (h *otelDataPointHasher) UpdateDataPoint(dp datapoints.DataPoint) {
	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.StartTimestamp()))
	hasher.Write(timestampBuf)

	hasher.Write([]byte(dp.Metric().Unit()))

	// TODO: handle geo attribute consistently as encoding logic
	mapHashSortedExcludeReservedAttrs(hasher, dp.Attributes(), elasticsearch.MappingHintsAttrKey)

	h.dpHash = hasher.Sum32()
}

func (h *otelDataPointHasher) HashKey() HashKey {
	return HashKey{
		resourceHash: h.resourceHash,
		scopeHash:    h.scopeHash,
		dpHash:       h.dpHash,
	}
}

// mapHashSortedExcludeReservedAttrs is mapHash but ignoring some reserved attributes and is independent of order in Map.
// e.g. index is already considered during routing and DS attributes do not need to be considered in hashing
func mapHashSortedExcludeReservedAttrs(hasher hash.Hash, m pcommon.Map, extra ...string) {
	type kv struct {
		k string
		v pcommon.Value
	}
	kvs := make([]kv, 0, m.Len())
	for k, v := range m.All() {
		switch k {
		case elasticsearch.DataStreamType, elasticsearch.DataStreamDataset, elasticsearch.DataStreamNamespace:
			continue
		}
		if slices.Contains(extra, k) {
			continue
		}
		kvs = append(kvs, kv{k: k, v: v})
	}

	slices.SortFunc(kvs, func(a, b kv) int {
		return cmp.Compare(a.k, b.k)
	})

	for _, kv := range kvs {
		hasher.Write([]byte(kv.k))
		valueHash(hasher, kv.v)
	}
}

func mapHash(hasher hash.Hash, m pcommon.Map) {
	for k, v := range m.All() {
		hasher.Write([]byte(k))
		valueHash(hasher, v)
	}
}

func valueHash(h hash.Hash, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		h.Write([]byte{0})
	case pcommon.ValueTypeStr:
		h.Write([]byte(v.Str()))
	case pcommon.ValueTypeBool:
		if v.Bool() {
			h.Write([]byte{1})
		} else {
			h.Write([]byte{0})
		}
	case pcommon.ValueTypeDouble:
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(v.Double()))
		h.Write(buf)
	case pcommon.ValueTypeInt:
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, uint64(v.Int()))
		h.Write(buf)
	case pcommon.ValueTypeBytes:
		h.Write(v.Bytes().AsRaw())
	case pcommon.ValueTypeMap:
		mapHash(h, v.Map())
	case pcommon.ValueTypeSlice:
		sliceHash(h, v.Slice())
	}
}

func sliceHash(h hash.Hash, s pcommon.Slice) {
	for i := 0; i < s.Len(); i++ {
		valueHash(h, s.At(i))
	}
}
