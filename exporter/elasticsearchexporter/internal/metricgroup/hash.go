// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricgroup // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metricgroup"

import (
	"cmp"
	"encoding/binary"
	"math"
	"slices"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// kv is a sortable key/value pair used when hashing maps in key-sorted order.
type kv struct {
	k string
	v pcommon.Value
}

var (
	// Package-level one-byte slices avoid allocating []byte{0}/[]byte{1} on
	// every empty/bool value write in the hash hot path.
	hashByteZero = []byte{0}
	hashByteOne  = []byte{1}
)

func resetKVs(kvs []kv, capHint int) []kv {
	clear(kvs)
	if cap(kvs) < capHint {
		return make([]kv, 0, capHint)
	}
	return kvs[:0]
}

func isReservedAttr(k string, extraExcludes []string) bool {
	switch k {
	case elasticsearch.DataStreamType, elasticsearch.DataStreamDataset, elasticsearch.DataStreamNamespace:
		return true
	}
	return slices.Contains(extraExcludes, k)
}

// mapHashSortedExcludeReservedAttrs is mapHash but ignoring some reserved attributes and is independent of order in Map.
// e.g. index is already considered during routing and DS attributes do not need to be considered in hashing
//
// TODO(carsonip): https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/39377
// Use opentelemetry-collector-contrib/pkg/pdatautil/hash.go when it can optionally exclude attributes
// We could have used it now but it'll involve creating a new Map and copying things over.
func mapHashSortedExcludeReservedAttrs(hasher *xxhash.Digest, kvs []kv, m pcommon.Map, extraExcludes ...string) []kv {
	kvs = appendSortedKVsExcludeReservedAttrs(resetKVs(kvs, m.Len()), m, extraExcludes...)
	for i := range kvs {
		_, _ = hasher.WriteString(kvs[i].k)
		valueHash(hasher, kvs[i].v)
	}
	return kvs
}

// sortedKVsExcludeReservedAttrs returns a newly allocated sorted kv slice. Used
// by ECSDataPointHasher.UpdateResource to cache resource attrs across HashKey
// calls.
func sortedKVsExcludeReservedAttrs(m pcommon.Map, extraExcludes ...string) []kv {
	return appendSortedKVsExcludeReservedAttrs(make([]kv, 0, m.Len()), m, extraExcludes...)
}

func appendSortedKVsExcludeReservedAttrs(kvs []kv, m pcommon.Map, extraExcludes ...string) []kv {
	for k, v := range m.All() {
		if isReservedAttr(k, extraExcludes) {
			continue
		}
		kvs = append(kvs, kv{k: k, v: v})
	}
	slices.SortFunc(kvs, func(a, b kv) int {
		return cmp.Compare(a.k, b.k)
	})
	return kvs
}

// writeMergedSortedKVs hashes the ECS merge of resource and datapoint attributes
// (datapoint wins on key conflict). Both inputs must already be sorted by key.
func writeMergedSortedKVs(hasher *xxhash.Digest, resourceKVs, dpKVs []kv) {
	i, j := 0, 0
	for i < len(resourceKVs) || j < len(dpKVs) {
		switch {
		case j >= len(dpKVs):
			_, _ = hasher.WriteString(resourceKVs[i].k)
			valueHash(hasher, resourceKVs[i].v)
			i++
		case i >= len(resourceKVs):
			_, _ = hasher.WriteString(dpKVs[j].k)
			valueHash(hasher, dpKVs[j].v)
			j++
		case resourceKVs[i].k < dpKVs[j].k:
			_, _ = hasher.WriteString(resourceKVs[i].k)
			valueHash(hasher, resourceKVs[i].v)
			i++
		case resourceKVs[i].k > dpKVs[j].k:
			_, _ = hasher.WriteString(dpKVs[j].k)
			valueHash(hasher, dpKVs[j].v)
			j++
		default:
			// Equal keys: datapoint overwrites resource.
			_, _ = hasher.WriteString(dpKVs[j].k)
			valueHash(hasher, dpKVs[j].v)
			i++
			j++
		}
	}
}

func mapHash(hasher *xxhash.Digest, m pcommon.Map) {
	for k, v := range m.All() {
		_, _ = hasher.WriteString(k)
		valueHash(hasher, v)
	}
}

func valueHash(h *xxhash.Digest, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		_, _ = h.Write(hashByteZero)
	case pcommon.ValueTypeStr:
		_, _ = h.WriteString(v.Str())
	case pcommon.ValueTypeBool:
		if v.Bool() {
			_, _ = h.Write(hashByteOne)
		} else {
			_, _ = h.Write(hashByteZero)
		}
	case pcommon.ValueTypeDouble:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(v.Double()))
		_, _ = h.Write(buf[:])
	case pcommon.ValueTypeInt:
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], uint64(v.Int()))
		_, _ = h.Write(buf[:])
	case pcommon.ValueTypeBytes:
		_, _ = h.Write(v.Bytes().AsRaw())
	case pcommon.ValueTypeMap:
		mapHash(h, v.Map())
	case pcommon.ValueTypeSlice:
		sliceHash(h, v.Slice())
	}
}

func sliceHash(h *xxhash.Digest, s pcommon.Slice) {
	for _, item := range s.All() {
		valueHash(h, item)
	}
}
