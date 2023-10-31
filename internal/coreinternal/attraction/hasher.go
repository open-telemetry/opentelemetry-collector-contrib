// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attraction // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"

import (
	"crypto/sha1" // #nosec
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	int64ByteSize   = 8
	float64ByteSize = 8
)

var (
	byteTrue  = [1]byte{1}
	byteFalse = [1]byte{0}
)

// Deprecated: [v0.75.0] use sha2Hasher instead.
// sha1Hasher hashes an AttributeValue using SHA1 and returns a
// hashed version of the attribute. In practice, this would mostly be used
// for string attributes but we support all types for completeness/correctness
// and eliminate any surprises.
func sha1Hasher(attr pcommon.Value) {
	var val []byte
	switch attr.Type() {
	case pcommon.ValueTypeStr:
		val = []byte(attr.Str())
	case pcommon.ValueTypeBool:
		if attr.Bool() {
			val = byteTrue[:]
		} else {
			val = byteFalse[:]
		}
	case pcommon.ValueTypeInt:
		val = make([]byte, int64ByteSize)
		binary.LittleEndian.PutUint64(val, uint64(attr.Int()))
	case pcommon.ValueTypeDouble:
		val = make([]byte, float64ByteSize)
		binary.LittleEndian.PutUint64(val, math.Float64bits(attr.Double()))
	}

	var hashed string
	if len(val) > 0 {
		// #nosec
		h := sha1.New()
		_, _ = h.Write(val)
		val = h.Sum(nil)
		hashedBytes := make([]byte, hex.EncodedLen(len(val)))
		hex.Encode(hashedBytes, val)
		hashed = string(hashedBytes)
	}

	attr.SetStr(hashed)
}

// sha2Hasher hashes an AttributeValue using SHA2-256 and returns a
// hashed version of the attribute. In practice, this would mostly be used
// for string attributes but we support all types for completeness/correctness
// and eliminate any surprises.
func sha2Hasher(attr pcommon.Value) {
	var val []byte
	switch attr.Type() {
	case pcommon.ValueTypeStr:
		val = []byte(attr.Str())
	case pcommon.ValueTypeBool:
		if attr.Bool() {
			val = byteTrue[:]
		} else {
			val = byteFalse[:]
		}
	case pcommon.ValueTypeInt:
		val = make([]byte, int64ByteSize)
		binary.LittleEndian.PutUint64(val, uint64(attr.Int()))
	case pcommon.ValueTypeDouble:
		val = make([]byte, float64ByteSize)
		binary.LittleEndian.PutUint64(val, math.Float64bits(attr.Double()))
	}

	var hashed string
	if len(val) > 0 {
		h := sha256.New()
		_, _ = h.Write(val)
		val = h.Sum(nil)
		hashedBytes := make([]byte, hex.EncodedLen(len(val)))
		hex.Encode(hashedBytes, val)
		hashed = string(hashedBytes)
	}

	attr.SetStr(hashed)
}
