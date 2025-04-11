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

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// dataPointHasher is an interface for hashing data points by their identity,
// for grouping into a single document.
type dataPointHasher interface {
	hashResource(pcommon.Resource) uint32
	hashScope(pcommon.InstrumentationScope) uint32
	hashDataPoint(datapoints.DataPoint) uint32
	hashCombined(pcommon.Resource, pcommon.InstrumentationScope, datapoints.DataPoint) uint32
}

func newDataPointHasher(mode MappingMode) dataPointHasher {
	switch mode {
	case MappingOTel:
		return otelDataPointHasher{}
	default:
		// Defaults to ECS for backward compatibility
		return ecsDataPointHasher{}
	}
}

// TODO use https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/internal/exp/metrics/identity

type (
	// ecsDataPointHasher solely relies on hashCombined because data point attributes overwrite resource attributes on merge.
	ecsDataPointHasher struct{}
	// otelDataPointHasher does not use hashCombined as resource, scope and data points are independent.
	otelDataPointHasher struct{}
)

func (h ecsDataPointHasher) hashResource(_ pcommon.Resource) uint32 {
	return 0
}

func (h ecsDataPointHasher) hashScope(_ pcommon.InstrumentationScope) uint32 {
	return 0
}

func (h ecsDataPointHasher) hashDataPoint(_ datapoints.DataPoint) uint32 {
	return 0
}

func (h ecsDataPointHasher) hashCombined(resource pcommon.Resource, _ pcommon.InstrumentationScope, dp datapoints.DataPoint) uint32 {
	merged := pcommon.NewMap()
	merged.EnsureCapacity(resource.Attributes().Len() + dp.Attributes().Len())
	resource.Attributes().CopyTo(merged)
	// scope attributes are ignored in ECS mode
	for k, v := range dp.Attributes().All() {
		v.CopyTo(merged.PutEmpty(k))
	}

	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	mapHashSortedExcludeReservedAttrs(hasher, merged)

	return hasher.Sum32()
}

func (h otelDataPointHasher) hashResource(resource pcommon.Resource) uint32 {
	hasher := fnv.New32a()
	mapHashSortedExcludeReservedAttrs(hasher, resource.Attributes(), elasticsearch.MappingHintsAttrKey)
	return hasher.Sum32()
}

func (h otelDataPointHasher) hashScope(scope pcommon.InstrumentationScope) uint32 {
	hasher := fnv.New32a()
	hasher.Write([]byte(scope.Name()))
	mapHashSortedExcludeReservedAttrs(hasher, scope.Attributes(), elasticsearch.MappingHintsAttrKey)
	return hasher.Sum32()
}

func (h otelDataPointHasher) hashDataPoint(dp datapoints.DataPoint) uint32 {
	hasher := fnv.New32a()

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.StartTimestamp()))
	hasher.Write(timestampBuf)

	hasher.Write([]byte(dp.Metric().Unit()))

	mapHashSortedExcludeReservedAttrs(hasher, dp.Attributes(), elasticsearch.MappingHintsAttrKey)

	return hasher.Sum32()
}

func (h otelDataPointHasher) hashCombined(_ pcommon.Resource, _ pcommon.InstrumentationScope, _ datapoints.DataPoint) uint32 {
	return 0
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
