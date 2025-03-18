// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
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
	hashDataPoint(pcommon.Resource, pcommon.InstrumentationScope, datapoints.DataPoint) uint32
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
	ecsDataPointHasher  struct{}
	otelDataPointHasher struct{}
)

func (h ecsDataPointHasher) hashDataPoint(resource pcommon.Resource, _ pcommon.InstrumentationScope, dp datapoints.DataPoint) uint32 {
	hasher := fnv.New32a()

	mapHashExcludeReservedAttrs(hasher, resource.Attributes())

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	mapHashExcludeReservedAttrs(hasher, dp.Attributes())

	return hasher.Sum32()
}

func (h otelDataPointHasher) hashDataPoint(resource pcommon.Resource, scope pcommon.InstrumentationScope, dp datapoints.DataPoint) uint32 {
	hasher := fnv.New32a()

	mapHashExcludeReservedAttrs(hasher, resource.Attributes(), elasticsearch.MappingHintsAttrKey)
	hasher.Write([]byte(scope.Name()))
	mapHashExcludeReservedAttrs(hasher, scope.Attributes(), elasticsearch.MappingHintsAttrKey)

	timestampBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.Timestamp()))
	hasher.Write(timestampBuf)

	binary.LittleEndian.PutUint64(timestampBuf, uint64(dp.StartTimestamp()))
	hasher.Write(timestampBuf)

	hasher.Write([]byte(dp.Metric().Unit()))

	mapHashExcludeReservedAttrs(hasher, dp.Attributes(), elasticsearch.MappingHintsAttrKey)

	return hasher.Sum32()
}

// mapHashExcludeReservedAttrs is mapHash but ignoring some reserved attributes.
// e.g. index is already considered during routing and DS attributes do not need to be considered in hashing
func mapHashExcludeReservedAttrs(hasher hash.Hash, m pcommon.Map, extra ...string) {
	for k, v := range m.All() {
		switch k {
		case elasticsearch.DataStreamType, elasticsearch.DataStreamDataset, elasticsearch.DataStreamNamespace:
			continue
		}
		if slices.Contains(extra, k) {
			continue
		}
		hasher.Write([]byte(k))
		valueHash(hasher, v)
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
