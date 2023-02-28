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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type readerConfig struct {
	fingerprintSize int
	maxLogSize      int
	emit            EmitFunc
}

// Reader manages a single file
type Reader struct {
	*zap.SugaredLogger `json:"-"` // json tag excludes embedded fields from storage
	*readerConfig
	splitFunc bufio.SplitFunc
	encoding  helper.Encoding

	Fingerprint    *Fingerprint
	Offset         int64
	generation     int
	file           *os.File
	fileAttributes *FileAttributes
	eof            bool
	header         *header
}

// offsetToEnd sets the starting offset
func (r *Reader) offsetToEnd() error {
	info, err := r.file.Stat()
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	r.Offset = info.Size()
	return nil
}

// ReadToEnd will read until the end of the file
func (r *Reader) ReadToEnd(ctx context.Context) {
	if r.header != nil && !r.header.Finalized() {
		r.header.ReadHeader(ctx, r.file, r.encoding, r.fileAttributes)
		// Don't read log entries if the header has not yet been finalized
		// (we are still waiting for the full header to be read).
		if !r.header.Finalized() {
			return
		}

		// Set r to the end of the headers if our current offset is within the header logs
		if r.Offset < r.header.Offset() {
			if err := r.readHeaderFingerprint(); err != nil {
				r.Errorw("Failed to read header into fingerprint.", zap.Error(err))
			}
			r.Offset = r.header.Offset()
		}
	}

	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.Errorw("Failed to seek", zap.Error(err))
		return
	}

	scanner := NewPositionalScanner(r, r.maxLogSize, r.Offset, r.splitFunc)

	// Iterate over the tokenized file, emitting entries as we go
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ok := scanner.Scan()
		if !ok {
			r.eof = true
			if err := scanner.getError(); err != nil {
				// If Scan returned an error then we are not guaranteed to be at the end of the file
				r.eof = false
				r.Errorw("Failed during scan", zap.Error(err))
			}
			break
		}

		token, err := r.encoding.Decode(scanner.Bytes())
		if err != nil {
			r.Errorw("decode: %w", zap.Error(err))
		} else {
			r.emit(ctx, r.fileAttributes, token)
		}

		r.Offset = scanner.Pos()
	}
}

// Close will close the file
func (r *Reader) Close() {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.Debugw("Problem closing reader", zap.Error(err))
		}
	}

	if r.header != nil {
		if err := r.header.Shutdown(); err != nil {
			r.Warnw("Problem shutting down header pipeline", zap.Error(err))
		}
	}
}

// Read from the file and update the fingerprint if necessary
func (r *Reader) Read(dst []byte) (int, error) {
	// Skip if fingerprint is already built
	// or if fingerprint is behind Offset
	if len(r.Fingerprint.FirstBytes) == r.fingerprintSize || int(r.Offset) > len(r.Fingerprint.FirstBytes) {
		return r.file.Read(dst)
	}
	n, err := r.file.Read(dst)
	appendCount := min0(n, r.fingerprintSize-int(r.Offset))
	// return for n == 0 or r.Offset >= r.fileInput.fingerprintSize
	if appendCount == 0 {
		return n, err
	}

	// for appendCount==0, the following code would add `0` to fingerprint
	r.Fingerprint.FirstBytes = append(r.Fingerprint.FirstBytes[:r.Offset], dst[:appendCount]...)
	return n, err
}

func (r *Reader) readHeaderFingerprint() error {
	// Check if fingerprint already has the header bytes
	readAmnt := min0(int(r.header.Offset()), r.fingerprintSize)
	if len(r.Fingerprint.FirstBytes) >= readAmnt {
		return nil
	}

	initialFpBytes := make([]uint8, readAmnt, r.fingerprintSize)

	if _, err := r.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to start of file: %w", err)
	}

	if _, err := r.file.Read(initialFpBytes); err != nil {
		return fmt.Errorf("failed to read from file: %w", err)
	}

	r.Fingerprint.FirstBytes = initialFpBytes
	return nil
}

func min0(a, b int) int {
	if a < 0 || b < 0 {
		return 0
	}
	if a < b {
		return a
	}
	return b
}
