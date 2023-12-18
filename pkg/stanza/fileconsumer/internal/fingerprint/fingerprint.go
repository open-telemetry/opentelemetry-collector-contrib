// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fingerprint // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"

import (
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"os"
)

const DefaultSize = 1000 // bytes

const MinSize = 16 // bytes

// Fingerprint is used to identify a file
// A file's fingerprint is the first N bytes of the file
type Fingerprint struct {
	FirstBytes   []byte
	HashBytes    uint64
	BytesUsed    int
	HashInstance hash.Hash64
}

func (f Fingerprint) AddHash() {
	if f.FirstBytes == nil {
		return
	}
	h := fnv.New64()
	h.Write(f.FirstBytes)
	hashed := h.Sum64()
	f.HashBytes = hashed
	f.BytesUsed = len(f.FirstBytes)
	f.HashInstance = h
}

// New creates a new fingerprint from an open file
func New(file *os.File, size int) (*Fingerprint, error) {
	buf := make([]byte, size)
	n, err := file.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading fingerprint bytes: %w", err)
	}
	fBytes := buf[:n]

	h := fnv.New64()
	h.Write(fBytes)
	hashed := h.Sum64()

	fp := &Fingerprint{
		FirstBytes:   fBytes,
		HashBytes:    hashed,
		BytesUsed:    len(fBytes),
		HashInstance: h,
	}

	return fp, nil
}

// Copy creates a new copy of the fingerprint
func (f Fingerprint) Copy() *Fingerprint {
	buf := make([]byte, len(f.FirstBytes), cap(f.FirstBytes))
	n := copy(buf, f.FirstBytes)
	return &Fingerprint{
		FirstBytes:   buf[:n],
		HashBytes:    f.HashBytes,
		BytesUsed:    f.BytesUsed,
		HashInstance: f.HashInstance,
	}
}

// UpdateFingerPrint will update fingerprint with new bytes
//func (f *Fingerprint) UpdateFingerPrint(offset int64, appendBytes []byte) {
//	if f.FirstBytes == nil {
//		f.FirstBytes = appendBytes
//	} else {
//		f.FirstBytes = append(f.FirstBytes[:offset], appendBytes...)
//	}
//	h := fnv.New64()
//	h.Write(f.FirstBytes)
//	hash := h.Sum64()
//	f.HashBytes = hash
//	f.BytesUsed = len(f.FirstBytes)
//}

// Equal returns true if the fingerprints have the same FirstBytes,
// false otherwise. This does not compare other aspects of the fingerprints
// because the primary purpose of a fingerprint is to convey a unique
// identity, and the HashBytes field contributes to this goal.
func (f Fingerprint) Equal(other *Fingerprint) bool {
	return f.HashBytes == other.HashBytes
}

// StartsWith returns true if the fingerprints are the same
// or if the new fingerprint starts with the old one
// This is important functionality for tracking new files,
// since their initial size is typically less than that of
// a fingerprint. As the file grows, its fingerprint is updated
// until it reaches a maximum size, as configured on the operator
func (f Fingerprint) StartsWith(old *Fingerprint) bool {
	lenOld := old.BytesUsed
	if lenOld == 0 {
		return false
	}
	lenCurrent := len(f.FirstBytes)
	if lenOld > lenCurrent {
		return false
	}
	if f.BytesUsed == old.BytesUsed {
		return f.HashBytes == old.HashBytes
	}
	h := fnv.New64()
	h.Write(f.FirstBytes[:lenOld])
	hash := h.Sum64()
	return old.HashBytes == hash
}

// IsMaxSize checks to see if fingerprint has reached its maxed size
func (f Fingerprint) IsMaxSize(maxFingerprintSize int, offset int64) bool {
	return f.BytesUsed == maxFingerprintSize || int(offset) > f.BytesUsed
}
