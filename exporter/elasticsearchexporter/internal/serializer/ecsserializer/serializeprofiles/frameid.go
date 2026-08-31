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
	var fID frameID

	bytes, err := base64.RawURLEncoding.DecodeString(frameEncoded)
	if err != nil {
		return fID, fmt.Errorf("failed to decode frameID %v: %w", frameEncoded, err)
	}

	return newFrameIDFromBytes(bytes)
}

// newFrameIDFromBytes creates a new FrameID from a byte array of length 24.
func newFrameIDFromBytes(bytes []byte) (frameID, error) {
	var fID frameID
	var err error

	if len(bytes) != 24 {
		return fID, fmt.Errorf("unexpected frameID size (expected 24 bytes): %d",
			len(bytes))
	}

	if fID.fileID, err = libpf.FileIDFromBytes(bytes[0:16]); err != nil {
		return fID, fmt.Errorf("failed to create fileID from bytes: %w", err)
	}

	fID.addressOrLineno = libpf.AddressOrLineno(binary.BigEndian.Uint64(bytes[16:24]))

	return fID, nil
}

// Bytes returns the frameid as byte sequence.
func (f frameID) Bytes() []byte {
	// Using frameID := make([byte, 24]) here makes the function ~5% slower.
	var fID [24]byte

	copy(fID[:], f.fileID.Bytes())
	binary.BigEndian.PutUint64(fID[16:], uint64(f.addressOrLineno))
	return fID[:]
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
