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

package pdatautil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestMapHash_Equal(t *testing.T) {
	tests := []struct {
		name  string
		maps  []pcommon.Map
		equal bool
	}{
		{
			name: "different_maps",
			maps: func() []pcommon.Map {
				m := make([]pcommon.Map, 29)
				for i := 0; i < len(m); i++ {
					m[i] = pcommon.NewMap()
				}
				m[1].PutStr("k", "")
				m[2].PutStr("k", "v")
				m[3].PutStr("k1", "v1")
				m[4].PutBool("k", false)
				m[5].PutBool("k", true)
				m[6].PutInt("k", 0)
				m[7].PutInt("k", 1)
				m[8].PutDouble("k", 0)
				m[9].PutDouble("k", 1)
				m[10].PutEmpty("k")

				m[11].PutStr("k1", "val")
				m[11].PutStr("k2", "val")
				m[12].PutStr("k1", "va")
				m[12].PutStr("lk2", "val")

				m[13].PutEmptySlice("k")
				m[14].PutEmptySlice("k").AppendEmpty()
				m[15].PutEmptySlice("k").AppendEmpty().SetStr("")
				m[16].PutEmptySlice("k").AppendEmpty().SetStr("v")
				sl1 := m[17].PutEmptySlice("k")
				sl1.AppendEmpty().SetStr("v1")
				sl1.AppendEmpty().SetStr("v2")
				sl2 := m[18].PutEmptySlice("k")
				sl2.AppendEmpty().SetStr("v2")
				sl2.AppendEmpty().SetStr("v1")

				m[19].PutEmptyBytes("k")
				m[20].PutEmptyBytes("k").FromRaw([]byte{0})
				m[21].PutEmptyBytes("k").FromRaw([]byte{1})

				m[22].PutEmptyMap("k")
				m[23].PutEmptyMap("k").PutStr("k", "")
				m[24].PutEmptyMap("k").PutBool("k", false)
				m[25].PutEmptyMap("k").PutEmptyMap("")
				m[26].PutEmptyMap("k").PutEmptyMap("k")

				m[27].PutStr("k1", "v1")
				m[27].PutStr("k2", "v2")
				m[28].PutEmptyMap("k0").PutStr("k1", "v1")
				m[28].PutStr("k2", "v2")

				return m
			}(),
			equal: false,
		},
		{
			name: "empty_maps",
			maps: func() []pcommon.Map {
				return []pcommon.Map{pcommon.NewMap(), pcommon.NewMap()}
			}(),
			equal: true,
		},
		{
			name: "same_maps_different_order",
			maps: func() []pcommon.Map {
				m := []pcommon.Map{pcommon.NewMap(), pcommon.NewMap()}
				m[0].PutStr("k1", "v1")
				m[0].PutInt("k2", 1)
				m[0].PutDouble("k3", 1)
				m[0].PutBool("k4", true)
				m[0].PutEmptyBytes("k5").FromRaw([]byte("abc"))
				sl := m[0].PutEmptySlice("k6")
				sl.AppendEmpty().SetStr("str")
				sl.AppendEmpty().SetBool(true)
				m0 := m[0].PutEmptyMap("k")
				m0.PutInt("k1", 1)
				m0.PutDouble("k2", 10)

				m1 := m[1].PutEmptyMap("k")
				m1.PutDouble("k2", 10)
				m1.PutInt("k1", 1)
				m[1].PutEmptyBytes("k5").FromRaw([]byte("abc"))
				m[1].PutBool("k4", true)
				sl = m[1].PutEmptySlice("k6")
				sl.AppendEmpty().SetStr("str")
				sl.AppendEmpty().SetBool(true)
				m[1].PutInt("k2", 1)
				m[1].PutStr("k1", "v1")
				m[1].PutDouble("k3", 1)

				return m
			}(),
			equal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < len(tt.maps); i++ {
				for j := i + 1; j < len(tt.maps); j++ {
					if tt.equal {
						assert.Equal(t, MapHash(tt.maps[i]), MapHash(tt.maps[j]),
							"maps %d %v and %d %v must have the same hash", i, tt.maps[i].AsRaw(), j, tt.maps[j].AsRaw())
					} else {
						assert.NotEqual(t, MapHash(tt.maps[i]), MapHash(tt.maps[j]),
							"maps %d %v and %d %v must have different hashes", i, tt.maps[i].AsRaw(), j, tt.maps[j].AsRaw())
					}
				}
			}
		})
	}
}

func BenchmarkMapHashFourItems(b *testing.B) {
	m := pcommon.NewMap()
	m.PutStr("test-string-key2", "test-value-2")
	m.PutStr("test-string-key1", "test-value-1")
	m.PutInt("test-int-key", 123)
	m.PutBool("test-bool-key", true)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MapHash(m)
	}
}

func BenchmarkMapHashEightItems(b *testing.B) {
	m := pcommon.NewMap()
	m.PutStr("test-string-key2", "test-value-2")
	m.PutStr("test-string-key1", "test-value-1")
	m.PutInt("test-int-key", 123)
	m.PutBool("test-bool-key", true)
	m.PutStr("test-string-key3", "test-value-3")
	m.PutDouble("test-double-key2", 22.123)
	m.PutDouble("test-double-key1", 11.123)
	m.PutEmptyBytes("test-bytes-key").FromRaw([]byte("abc"))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MapHash(m)
	}
}

func BenchmarkMapHashWithEmbeddedSliceAndMap(b *testing.B) {
	m := pcommon.NewMap()
	m.PutStr("test-string-key2", "test-value-2")
	m.PutStr("test-string-key1", "test-value-1")
	m.PutInt("test-int-key", 123)
	m.PutBool("test-bool-key", true)
	m.PutStr("test-string-key3", "test-value-3")
	m.PutDouble("test-double-key2", 22.123)
	m.PutDouble("test-double-key1", 11.123)
	m.PutEmptyBytes("test-bytes-key").FromRaw([]byte("abc"))
	m1 := m.PutEmptyMap("test-map-key")
	m1.PutStr("test-embedded-string-key", "test-embedded-string-value")
	m1.PutDouble("test-embedded-double-key", 22.123)
	m1.PutInt("test-embedded-int-key", 234)
	sl := m.PutEmptySlice("test-slice-key")
	sl.AppendEmpty().SetStr("test-slice-string-1")
	sl.AppendEmpty().SetStr("test-slice-string-2")
	sl.AppendEmpty().SetStr("test-slice-string-3")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MapHash(m)
	}
}
