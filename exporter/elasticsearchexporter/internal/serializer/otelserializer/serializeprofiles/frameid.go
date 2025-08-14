// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer/serializeprofiles"

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// frameID represents a frame as an address in an executable file
// or as a line in a source code file.
type frameID struct {
	// fileID is the fileID of the frame
	fileID libpf.FileID

	// addressOrLineno is the address or lineno of the frame
	addressOrLineno libpf.AddressOrLineno
}

// newFrameID creates a new FrameID from the fileId and address or line.
func newFrameID(fileID libpf.FileID, addressOrLineno libpf.AddressOrLineno) frameID {
	return frameID{
		fileID:          fileID,
		addressOrLineno: addressOrLineno,
	}
}

// newFrameIDFromString creates a new FrameID from its base64 string representation.
func newFrameIDFromString(frameEncoded string) (frameID, error) {
	var frameID frameID

	bytes, err := base64.RawURLEncoding.DecodeString(frameEncoded)
	if err != nil {
		return frameID, fmt.Errorf("failed to decode frameID %v: %v", frameEncoded, err)
	}

	return newFrameIDFromBytes(bytes)
}

// newFrameIDFromBytes creates a new FrameID from a byte array of length 24.
func newFrameIDFromBytes(bytes []byte) (frameID, error) {
	var frameID frameID
	var err error

	if len(bytes) != 24 {
		return frameID, fmt.Errorf("unexpected frameID size (expected 24 bytes): %d",
			len(bytes))
	}

	if frameID.fileID, err = libpf.FileIDFromBytes(bytes[0:16]); err != nil {
		return frameID, fmt.Errorf("failed to create fileID from bytes: %v", err)
	}

	frameID.addressOrLineno = libpf.AddressOrLineno(binary.BigEndian.Uint64(bytes[16:24]))

	return frameID, nil
}

// Bytes returns the frameid as byte sequence.
func (f frameID) Bytes() []byte {
	// Using frameID := make([byte, 24]) here makes the function ~5% slower.
	var frameID [24]byte

	copy(frameID[:], f.fileID.Bytes())
	binary.BigEndian.PutUint64(frameID[16:], uint64(f.addressOrLineno))
	return frameID[:]
}

// String returns the base64 encoded representation.
func (f frameID) String() string {
	return base64.RawURLEncoding.EncodeToString(f.Bytes())
}

// FileID returns the fileID part of the frameID.
func (f frameID) FileID() libpf.FileID {
	return f.fileID
}

// AddressOrLine returns the addressOrLine part of the frameID.
func (f frameID) AddressOrLine() libpf.AddressOrLineno {
	return f.addressOrLineno
}
