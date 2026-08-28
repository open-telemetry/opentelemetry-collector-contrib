// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"bytes"
	"encoding/base64"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// runLengthEncodeFrameTypesReverseTo is a specialized run-length encoder for
// a reversed array of FrameType elements.
//
// The output is a binary stream of 1-byte length and the binary representation of the
// object.
// E.g. an uint8 array like ['a', 'a', 'x', 'x', 'x', 'x', 'x'] is converted into
// the byte array [5, 'x', 2, 'a'].
//
// This function has been optimized to do zero heap allocations if dst has enough capacity.
func runLengthEncodeFrameTypesReverseTo(dst *bytes.Buffer, values []libpf.FrameType) {
	if len(values) == 0 {
		return
	}

	l := 1
	cur := values[len(values)-1]

	write := func() {
		dst.WriteByte(byte(l))
		dst.WriteByte(byte(cur))
	}

	for i := len(values) - 2; i >= 0; i-- {
		next := values[i]

		if next == cur && l < 255 {
			l++
			continue
		}

		write()
		l = 1
		cur = next
	}

	write()
}

// encodeFrameTypesTo applies run-length encoding to the frame types in reverse order
// and writes the results as base64url encoded string into dst.
//
// This function has been optimized to do zero heap allocations if dst has enough capacity.
func encodeFrameTypesTo(dst *bytes.Buffer, frameTypes []libpf.FrameType) {
	// Up to 255 consecutive identical frame types are converted into 2 bytes (binary).
	// Switching between frame types does not happen often, so 128 is more than enough
	// for the base64 representation, even to cover most corner cases.
	// The fallback will do a heap allocation for the rare cases that need more than 128 bytes.
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	runLengthEncodeFrameTypesReverseTo(buf, frameTypes)

	var tmp []byte
	tmplen := base64.RawURLEncoding.EncodedLen(buf.Len())
	if tmplen <= 128 {
		// Enforce stack allocation by using a fixed size.
		tmp = make([]byte, 128)
	} else {
		// Fall back to heap allocation.
		tmp = make([]byte, tmplen)
	}
	base64.RawURLEncoding.Encode(tmp, buf.Bytes())
	dst.Write(tmp[:tmplen])
}
