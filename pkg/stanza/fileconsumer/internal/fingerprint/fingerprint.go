// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fingerprint // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"

import (
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
)

const DefaultSize = 1000 // bytes

const MinSize = 16 // bytes

// Fingerprint is used to identify a file
// A file's fingerprint is the first N bytes of the file
type Fingerprint struct {
	firstBytes  []byte
	HashBytes   []byte
	BytesLength int
}

// New creates a new fingerprint from an open file
func New(file *os.File, size int) (*Fingerprint, error) {
	buf := make([]byte, size)
	//file.Seek(0, 0)
	n, err := file.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading fingerprint bytes: %w", err)
	}
	fBytes := buf[:n]

	h := fnv.New128a()
	// Write some data to the hash function.
	h.Write(fBytes)
	// Get the hash value.
	hash := h.Sum(nil)

	fp := &Fingerprint{
		firstBytes:  fBytes,
		HashBytes:   hash,
		BytesLength: len(fBytes),
	}

	return fp, nil
}

// Copy creates a new copy of the fingerprint
func (f Fingerprint) Copy() *Fingerprint {
	buf := make([]byte, len(f.firstBytes), cap(f.firstBytes))
	n := copy(buf, f.firstBytes)
	return &Fingerprint{
		firstBytes:  buf[:n],
		HashBytes:   f.HashBytes,
		BytesLength: f.BytesLength,
	}
}

func (f *Fingerprint) UpdateFingerPrint(offset int64, appendBytes []byte) {
	if f.firstBytes == nil {
		f.firstBytes = appendBytes
	} else {
		f.firstBytes = append(f.firstBytes[:offset], appendBytes...)
	}
	h := fnv.New128a()
	h.Write(f.firstBytes)
	hash := h.Sum(nil)
	f.HashBytes = hash
	f.BytesLength = len(f.firstBytes)
}

// Equal returns true if the fingerprints have the same FirstBytes,
// false otherwise. This does not compare other aspects of the fingerprints
// because the primary purpose of a fingerprint is to convey a unique
// identity, and only the FirstBytes field contributes to this goal.
func (f Fingerprint) Equal(other *Fingerprint) bool {
	return bytes.Equal(f.HashBytes, other.HashBytes)
}

// StartsWith returns true if the fingerprints are the same
// or if the new fingerprint starts with the old one
// This is important functionality for tracking new files,
// since their initial size is typically less than that of
// a fingerprint. As the file grows, its fingerprint is updated
// until it reaches a maximum size, as configured on the operator
func (f Fingerprint) StartsWith(old *Fingerprint) bool {
	l0 := old.BytesLength
	if l0 == 0 {
		return false
	}
	l1 := len(f.firstBytes)
	if l0 > l1 {
		return false
	}
	h := fnv.New128a()
	h.Write(f.firstBytes[:l0])
	hash := h.Sum(nil)
	return bytes.Equal(old.HashBytes, hash)
}
