// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricgroup // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metricgroup"

import (
	"cmp"
	"encoding/binary"
	"hash"
	"io"
	"math"
	"slices"
	"sync"

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

	// Pools keep HashKey/UpdateDataPoint at 0 allocs/op: previously every call
	// allocated a fresh []kv for sorting and a fresh xxhash.Digest.
	kvSlicePool = sync.Pool{
		New: func() any {
			s := make([]kv, 0, 64)
			return &s
		},
	}
	xxhashPool = sync.Pool{
		New: func() any {
			return xxhash.New()
		},
	}
)

func acquireKVSlice(capHint int) *[]kv {
	p := kvSlicePool.Get().(*[]kv)
	if cap(*p) < capHint {
		*p = make([]kv, 0, capHint)
	} else {
		*p = (*p)[:0]
	}
	return p
}

func releaseKVSlice(p *[]kv) {
	// Drop oversized buffers so a rare huge map does not pin memory in the pool.
	if p == nil || cap(*p) > 1024 {
		return
	}
	// Clear entries so pooled slices do not retain pcommon.Value references.
	clear(*p)
	*p = (*p)[:0]
	kvSlicePool.Put(p)
}

func acquireXXHash() *xxhash.Digest {
	return xxhashPool.Get().(*xxhash.Digest)
}

func releaseXXHash(d *xxhash.Digest) {
	d.Reset()
	xxhashPool.Put(d)
}

func isReservedAttr(k string, extraExcludes []string) bool {
	switch k {
	case elasticsearch.DataStreamType, elasticsearch.DataStreamDataset, elasticsearch.DataStreamNamespace:
		return true
	}
	return slices.Contains(extraExcludes, k)
}

// writeHashString prefers WriteString to avoid the []byte(string) allocation
// that hasher.Write([]byte(s)) performs. xxhash.Digest implements
// io.StringWriter.
func writeHashString(hasher hash.Hash, s string) {
	if sw, ok := hasher.(io.StringWriter); ok {
		_, _ = sw.WriteString(s)
		return
	}
	_, _ = hasher.Write([]byte(s))
}

// mapHashSortedExcludeReservedAttrs is mapHash but ignoring some reserved attributes and is independent of order in Map.
// e.g. index is already considered during routing and DS attributes do not need to be considered in hashing
//
// TODO(carsonip): https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/39377
// Use opentelemetry-collector-contrib/pkg/pdatautil/hash.go when it can optionally exclude attributes
// We could have used it now but it'll involve creating a new Map and copying things over.
func mapHashSortedExcludeReservedAttrs(hasher hash.Hash, m pcommon.Map, extraExcludes ...string) {
	// Sort into a pooled buffer instead of allocating []kv on every call.
	kvsPtr := acquireKVSlice(m.Len())
	kvs := appendSortedKVsExcludeReservedAttrs((*kvsPtr)[:0], m, extraExcludes...)
	*kvsPtr = kvs
	for i := range kvs {
		writeHashString(hasher, kvs[i].k)
		valueHash(hasher, kvs[i].v)
	}
	releaseKVSlice(kvsPtr)
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
func writeMergedSortedKVs(hasher hash.Hash, resourceKVs, dpKVs []kv) {
	i, j := 0, 0
	for i < len(resourceKVs) || j < len(dpKVs) {
		switch {
		case j >= len(dpKVs):
			writeHashString(hasher, resourceKVs[i].k)
			valueHash(hasher, resourceKVs[i].v)
			i++
		case i >= len(resourceKVs):
			writeHashString(hasher, dpKVs[j].k)
			valueHash(hasher, dpKVs[j].v)
			j++
		case resourceKVs[i].k < dpKVs[j].k:
			writeHashString(hasher, resourceKVs[i].k)
			valueHash(hasher, resourceKVs[i].v)
			i++
		case resourceKVs[i].k > dpKVs[j].k:
			writeHashString(hasher, dpKVs[j].k)
			valueHash(hasher, dpKVs[j].v)
			j++
		default:
			// Equal keys: datapoint overwrites resource.
			writeHashString(hasher, dpKVs[j].k)
			valueHash(hasher, dpKVs[j].v)
			i++
			j++
		}
	}
}

func mapHash(hasher hash.Hash, m pcommon.Map) {
	for k, v := range m.All() {
		writeHashString(hasher, k)
		valueHash(hasher, v)
	}
}

func valueHash(h hash.Hash, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeEmpty:
		_, _ = h.Write(hashByteZero)
	case pcommon.ValueTypeStr:
		writeHashString(h, v.Str())
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

func sliceHash(h hash.Hash, s pcommon.Slice) {
	for _, item := range s.All() {
		valueHash(h, item)
	}
}
