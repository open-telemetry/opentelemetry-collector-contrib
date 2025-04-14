// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricgroup // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metricgroup"

import (
	"cmp"
	"encoding/binary"
	"hash"
	"math"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// mapHashSortedExcludeReservedAttrs is mapHash but ignoring some reserved attributes and is independent of order in Map.
// e.g. index is already considered during routing and DS attributes do not need to be considered in hashing
//
// TODO(carsonip): https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/39377
// Use opentelemetry-collector-contrib/pkg/pdatautil/hash.go when it can optionally exclude attributes
// We could have used it now but it'll involve creating a new Map and copying things over.
func mapHashSortedExcludeReservedAttrs(hasher hash.Hash, m pcommon.Map, extraExcludes ...string) {
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
		if slices.Contains(extraExcludes, k) {
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
