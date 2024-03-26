// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fingerprint // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"

import (
	"bytes"
	"encoding/json"
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
	firstBytes     []byte
	firstBytesHash uint64
}

func New(first []byte) *Fingerprint {
	h := fnv.New64()
	_, _ = h.Write(first) // nolint:errcheck // fnv.Write never returns an error
	return &Fingerprint{firstBytes: first, firstBytesHash: h.Sum64()}
}

func NewFromFile(file *os.File, size int) (*Fingerprint, error) {
	buf := make([]byte, size)
	n, err := file.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading fingerprint bytes: %w", err)
	}
	return New(buf[:n]), nil
}

// Copy creates a new copy of the fingerprint
func (f Fingerprint) Copy() *Fingerprint {
	buf := make([]byte, len(f.firstBytes), cap(f.firstBytes))
	n := copy(buf, f.firstBytes)
	return New(buf[:n])
}

func (f *Fingerprint) Len() int {
	return len(f.firstBytes)
}

func (f *Fingerprint) Truncate(size int) {
	if size < 0 || size > len(f.firstBytes) {
		return
	}
	shorter := New(f.firstBytes[:size])
	*f = *shorter
}

// Equal returns true if the fingerprints have the same firstBytes,
// false otherwise. This does not compare other aspects of the fingerprints
// because the primary purpose of a fingerprint is to convey a unique
// identity, and only the firstBytes field contributes to this goal.
func (f *Fingerprint) Equal(other *Fingerprint) bool {
	if len(f.firstBytes) != len(other.firstBytes) {
		return false
	}
	return f.firstBytesHash == other.firstBytesHash
}

// StartsWith returns true if the fingerprints are the same
// or if the new fingerprint starts with the old one
// This is important functionality for tracking new files,
// since their initial size is typically less than that of
// a fingerprint. As the file grows, its fingerprint is updated
// until it reaches a maximum size, as configured on the operator
func (f *Fingerprint) StartsWith(other *Fingerprint) bool {
	if len(other.firstBytes) == 0 {
		return false
	}
	if len(f.firstBytes) < len(other.firstBytes) {
		return false
	}
	if len(f.firstBytes) == len(other.firstBytes) {
		return f.firstBytesHash == other.firstBytesHash
	}
	return bytes.Equal(f.firstBytes[:len(other.firstBytes)], other.firstBytes)
}

func (f *Fingerprint) MarshalJSON() ([]byte, error) {
	m := marshal{FirstBytes: f.firstBytes}
	return json.Marshal(&m)
}

func (f *Fingerprint) UnmarshalJSON(data []byte) error {
	m := new(marshal)
	if err := json.Unmarshal(data, m); err != nil {
		return err
	}
	f.firstBytes = m.FirstBytes
	h := fnv.New64()
	_, _ = h.Write(f.firstBytes) // nolint:errcheck
	f.firstBytesHash = h.Sum64()
	return nil
}

type marshal struct {
	FirstBytes []byte `json:"first_bytes"`
}
