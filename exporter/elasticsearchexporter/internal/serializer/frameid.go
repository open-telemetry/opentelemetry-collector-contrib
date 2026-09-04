// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

// FrameID represents a frame as an address in an executable file
// or as a line in a source code file.
type FrameID struct {
	fileID          libpf.FileID
	addressOrLineno libpf.AddressOrLineno
}

// NewFrameID creates a new FrameID from the fileId and address or line.
func NewFrameID(fileID libpf.FileID, addressOrLineno libpf.AddressOrLineno) FrameID {
	return FrameID{
		fileID:          fileID,
		addressOrLineno: addressOrLineno,
	}
}

// NewFrameIDFromString creates a new FrameID from its base64 string representation.
func NewFrameIDFromString(frameEncoded string) (FrameID, error) {
	var fID FrameID

	bytes, err := base64.RawURLEncoding.DecodeString(frameEncoded)
	if err != nil {
		return fID, fmt.Errorf("failed to decode frameID %v: %w", frameEncoded, err)
	}

	return NewFrameIDFromBytes(bytes)
}

// NewFrameIDFromBytes creates a new FrameID from a byte array of length 24.
func NewFrameIDFromBytes(bytes []byte) (FrameID, error) {
	var fID FrameID
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

// Bytes returns the FrameID as byte sequence.
func (f FrameID) Bytes() []byte {
	var fID [24]byte

	copy(fID[:], f.fileID.Bytes())
	binary.BigEndian.PutUint64(fID[16:], uint64(f.addressOrLineno))
	return fID[:]
}

// String returns the base64 encoded representation.
func (f FrameID) String() string {
	return base64.RawURLEncoding.EncodeToString(f.Bytes())
}

// FileID returns the fileID part of the FrameID.
func (f FrameID) FileID() libpf.FileID {
	return f.fileID
}

// AddressOrLine returns the addressOrLine part of the FrameID.
func (f FrameID) AddressOrLine() libpf.AddressOrLineno {
	return f.addressOrLineno
}
