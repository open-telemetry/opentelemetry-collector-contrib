// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

const DefaultFingerprintSize = 1000 // bytes
const MinFingerprintSize = 16       // bytes

// Fingerprint is used to identify a file
// A file's fingerprint is the first N bytes of the file,
// where N is the fingerprintSize on the file_input operator
type Fingerprint struct {
	FirstBytes []byte
}

// NewFingerprint creates a new fingerprint from an open file
func (f *Input) NewFingerprint(file *os.File) (*Fingerprint, error) {
	buf := make([]byte, f.fingerprintSize)

	n, err := file.ReadAt(buf, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("reading fingerprint bytes: %w", err)
	}

	fp := &Fingerprint{
		FirstBytes: buf[:n],
	}

	return fp, nil
}

// Copy creates a new copy of the fingerprint
func (f Fingerprint) Copy() *Fingerprint {
	buf := make([]byte, len(f.FirstBytes), cap(f.FirstBytes))
	n := copy(buf, f.FirstBytes)
	return &Fingerprint{
		FirstBytes: buf[:n],
	}
}

// StartsWith returns true if the fingerprints are the same
// or if the new fingerprint starts with the old one
// This is important functionality for tracking new files,
// since their initial size is typically less than that of
// a fingerprint. As the file grows, its fingerprint is updated
// until it reaches a maximum size, as configured on the operator
func (f Fingerprint) StartsWith(old *Fingerprint) bool {
	l0 := len(old.FirstBytes)
	if l0 == 0 {
		return false
	}
	l1 := len(f.FirstBytes)
	if l0 > l1 {
		return false
	}
	return bytes.Equal(old.FirstBytes[:l0], f.FirstBytes[:l0])
}
