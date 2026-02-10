// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attraction // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"

import (
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

// sha2Hasher hashes an AttributeValue using SHA2-256 and returns a
// hashed version of the attribute. In practice, this would mostly be used
// for string attributes, but we support all types for completeness/correctness
// and eliminate any surprises.
func sha2Hasher(attr pcommon.Value) {
	var sum [sha256.Size]byte
	ok := true

	switch attr.Type() {
	case pcommon.ValueTypeStr:
		sum = sha256.Sum256([]byte(attr.Str()))
	case pcommon.ValueTypeBool:
		if attr.Bool() {
			sum = sha256.Sum256(byteTrue[:])
		} else {
			sum = sha256.Sum256(byteFalse[:])
		}
	case pcommon.ValueTypeInt:
		var b [int64ByteSize]byte
		binary.LittleEndian.PutUint64(b[:], uint64(attr.Int()))
		sum = sha256.Sum256(b[:])
	case pcommon.ValueTypeDouble:
		var b [float64ByteSize]byte
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(attr.Double()))
		sum = sha256.Sum256(b[:])
	default:
		// No-op for empty, maps, slices, bytes.
		ok = false
	}

	if !ok {
		attr.SetStr("")
		return
	}

	// Hex encoding is always 64 bytes for SHA-256.
	var hashedBytes [sha256.Size * 2]byte
	hex.Encode(hashedBytes[:], sum[:])
	attr.SetStr(string(hashedBytes[:]))
}
