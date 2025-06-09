// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fingerprint // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.opentelemetry.io/collector/featuregate"
)

const DefaultSize = 1000 // bytes

const MinSize = 16 // bytes

var DecompressedFingerprintFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"filelog.decompressFingerprint",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("Computes fingerprint for compressed files by decompressing its data"),
	featuregate.WithRegisterFromVersion("v0.128.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/40256"),
)

// Fingerprint is used to identify a file
// A file's fingerprint is the first N bytes of the file
type Fingerprint struct {
	firstBytes []byte
}

func New(first []byte) *Fingerprint {
	return &Fingerprint{firstBytes: first}
}

// NewFromFile computes fingerprint of the given file using first 'N' bytes
// Set decompressData to true to compute fingerprint of compressed files by decompressing its data first
func NewFromFile(file *os.File, size int, decompressData bool) (*Fingerprint, error) {
	buf := make([]byte, size)
	if DecompressedFingerprintFeatureGate.IsEnabled() {
		if decompressData {
			if hasGzipExtension(file.Name()) {
				// If the file is of compressed type, uncompress the data before creating its fingerprint
				uncompressedData, err := gzip.NewReader(file)
				if err != nil {
					return nil, fmt.Errorf("error uncompressing gzip file: %w", err)
				}
				defer uncompressedData.Close()

				n, err := uncompressedData.Read(buf)
				if err != nil && !errors.Is(err, io.EOF) {
					return nil, fmt.Errorf("error reading fingerprint bytes: %w", err)
				}
				return New(buf[:n]), nil
			}
		}
	}

	n, err := file.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading fingerprint bytes: %w", err)
	}
	return New(buf[:n]), nil
}

func hasGzipExtension(filename string) bool {
	return filepath.Ext(filename) == ".gz"
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

// Equal returns true if the fingerprints have the same FirstBytes,
// false otherwise. This does not compare other aspects of the fingerprints
// because the primary purpose of a fingerprint is to convey a unique
// identity, and only the FirstBytes field contributes to this goal.
func (f Fingerprint) Equal(other *Fingerprint) bool {
	l0 := len(other.firstBytes)
	l1 := len(f.firstBytes)
	if l0 != l1 {
		return false
	}
	for i := 0; i < l0; i++ {
		if other.firstBytes[i] != f.firstBytes[i] {
			return false
		}
	}
	return true
}

// StartsWith returns true if the fingerprints are the same
// or if the new fingerprint starts with the old one
// This is important functionality for tracking new files,
// since their initial size is typically less than that of
// a fingerprint. As the file grows, its fingerprint is updated
// until it reaches a maximum size, as configured on the operator
func (f Fingerprint) StartsWith(old *Fingerprint) bool {
	l0 := len(old.firstBytes)
	if l0 == 0 {
		return false
	}
	l1 := len(f.firstBytes)
	if l0 > l1 {
		return false
	}
	return bytes.Equal(old.firstBytes[:l0], f.firstBytes[:l0])
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
	return nil
}

type marshal struct {
	FirstBytes []byte `json:"first_bytes"`
}
