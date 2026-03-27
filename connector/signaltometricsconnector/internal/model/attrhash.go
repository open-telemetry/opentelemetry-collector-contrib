// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/model"

import (
	"encoding/binary"
	"math"
	"sync"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// attrHashBuf is a pooled byte buffer used to compute a 128-bit hash of a
// filtered attribute set without allocating a pcommon.Map.
type attrHashBuf struct {
	buf []byte
}

var attrHashBufPool = sync.Pool{
	New: func() any {
		return &attrHashBuf{buf: make([]byte, 0, 256)}
	},
}

// sum128 returns a 128-bit hash of b.buf using the same two-pass xxhash
// algorithm as pdatautil.MapHash.
func (b *attrHashBuf) sum128() [16]byte {
	var d xxhash.Digest
	var result [16]byte
	d.Reset()
	_, _ = d.Write(b.buf)
	d.Sum(result[:0])
	_, _ = d.Write([]byte{0})
	d.Sum(result[:8])
	return result
}

// appendAttrValue appends a type-tagged encoding of v to buf.
func appendAttrValue(buf []byte, v pcommon.Value) []byte {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		buf = append(buf, 'S')
		s := v.Str()
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
		buf = append(buf, s...)
	case pcommon.ValueTypeInt:
		buf = append(buf, 'I')
		buf = binary.LittleEndian.AppendUint64(buf, uint64(v.Int()))
	case pcommon.ValueTypeDouble:
		buf = append(buf, 'D')
		buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(v.Double()))
	case pcommon.ValueTypeBool:
		buf = append(buf, 'B')
		if v.Bool() {
			buf = append(buf, 1)
		} else {
			buf = append(buf, 0)
		}
	default:
		// bytes, map, slice: encode as string representation (rare for attributes)
		buf = append(buf, 'O')
		s := v.AsString()
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(s)))
		buf = append(buf, s...)
	}
	return buf
}
