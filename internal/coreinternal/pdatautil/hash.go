// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pdatautil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/pdatautil"

import (
	"encoding/binary"
	"hash"
	"math"
	"sort"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var (
	extraByte       = []byte{'\xf3'}
	keyPrefix       = []byte{'\xf4'}
	valEmpty        = []byte{'\xf5'}
	valBytesPrefix  = []byte{'\xf6'}
	valStrPrefix    = []byte{'\xf7'}
	valBoolTrue     = []byte{'\xf8'}
	valBoolFalse    = []byte{'\xf9'}
	valIntPrefix    = []byte{'\xfa'}
	valDoublePrefix = []byte{'\xfb'}
	valMapPrefix    = []byte{'\xfc'}
	valMapSuffix    = []byte{'\xfd'}
	valSlicePrefix  = []byte{'\xfe'}
	valSliceSuffix  = []byte{'\xff'}
)

// MapHash return a hash for the provided map.
// Maps with the same underlying key/value pairs in different order produce the same deterministic hash value.
func MapHash(m pcommon.Map) [16]byte {
	h := xxhash.New()
	writeMapHash(h, m)
	return hashSum128(h)
}

// ValueHash return a hash for the provided pcommon.Value.
func ValueHash(v pcommon.Value) [16]byte {
	h := xxhash.New()
	writeValueHash(h, v)
	return hashSum128(h)
}

func writeMapHash(h hash.Hash, m pcommon.Map) {
	keys := make([]string, 0, m.Len())
	m.Range(func(k string, v pcommon.Value) bool {
		keys = append(keys, k)
		return true
	})
	sort.Strings(keys)
	for _, k := range keys {
		v, _ := m.Get(k)
		h.Write(keyPrefix)
		h.Write([]byte(k))
		writeValueHash(h, v)
	}
}

func writeSliceHash(h hash.Hash, sl pcommon.Slice) {
	for i := 0; i < sl.Len(); i++ {
		writeValueHash(h, sl.At(i))
	}
}

func writeValueHash(h hash.Hash, v pcommon.Value) {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		h.Write(valStrPrefix)
		h.Write([]byte(v.Str()))
	case pcommon.ValueTypeBool:
		if v.Bool() {
			h.Write(valBoolTrue)
		} else {
			h.Write(valBoolFalse)
		}
	case pcommon.ValueTypeInt:
		h.Write(valIntPrefix)
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(v.Int()))
		h.Write(b)
	case pcommon.ValueTypeDouble:
		h.Write(valDoublePrefix)
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, math.Float64bits(v.Double()))
		h.Write(b)
	case pcommon.ValueTypeMap:
		h.Write(valMapPrefix)
		writeMapHash(h, v.Map())
		h.Write(valMapSuffix)
	case pcommon.ValueTypeSlice:
		h.Write(valSlicePrefix)
		writeSliceHash(h, v.Slice())
		h.Write(valSliceSuffix)
	case pcommon.ValueTypeBytes:
		h.Write(valBytesPrefix)
		h.Write(v.Bytes().AsRaw())
	case pcommon.ValueTypeEmpty:
		h.Write(valEmpty)
	}
}

// hashSum128 returns a [16]byte hash sum.
func hashSum128(h hash.Hash) [16]byte {
	b := make([]byte, 0, 16)
	b = h.Sum(b)

	// Append an extra byte to generate another part of the hash sum
	_, _ = h.Write(extraByte)
	b = h.Sum(b)

	res := [16]byte{}
	copy(res[:], b)
	return res
}
