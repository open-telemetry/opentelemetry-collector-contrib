// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
)

type Config struct {
	FingerprintSize         int
	MaxLogSize              int
	Emit                    emit.Callback
	IncludeFileName         bool
	IncludeFilePath         bool
	IncludeFileNameResolved bool
	IncludeFilePathResolved bool
}

type Metadata struct {
	Fingerprint     *fingerprint.Fingerprint
	Offset          int64
	FileAttributes  map[string]any
	HeaderFinalized bool
}

// Reader manages a single file
type Reader struct {
	*zap.SugaredLogger
	*Config
	*Metadata
	FileName      string
	EOF           bool
	file          *os.File
	lineSplitFunc bufio.SplitFunc
	splitFunc     bufio.SplitFunc
	decoder       *decode.Decoder
	headerReader  *header.Reader
	processFunc   emit.Callback
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

func (r *Reader) NewFingerprintFromFile() (*fingerprint.Fingerprint, error) {
	if r.file == nil {
		return nil, errors.New("file is nil")
	}
	return fingerprint.New(r.file, r.FingerprintSize)
}

// ReadToEnd will read until the end of the file
func (r *Reader) ReadToEnd(ctx context.Context) {
	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.Errorw("Failed to seek", zap.Error(err))
		return
	}

	s := scanner.New(r, r.MaxLogSize, scanner.DefaultBufferSize, r.Offset, r.splitFunc)

	// Iterate over the tokenized file, emitting entries as we go
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ok := s.Scan()
		if !ok {
			r.EOF = true
			if err := s.Error(); err != nil {
				// If Scan returned an error then we are not guaranteed to be at the end of the file
				r.EOF = false
				r.Errorw("Failed during scan", zap.Error(err))
			}
			break
		}

		token, err := r.decoder.Decode(s.Bytes())
		if err != nil {
			r.Errorw("decode: %w", zap.Error(err))
		} else if err := r.processFunc(ctx, token, r.FileAttributes); err != nil {
			if errors.Is(err, header.ErrEndOfHeader) {
				r.finalizeHeader()

				// Now that the header is consumed, use the normal split and process functions.
				// Recreate the scanner with the normal split func.
				// Do not use the updated offset from the old scanner, as the most recent token
				// could be split differently with the new splitter.
				r.splitFunc = r.lineSplitFunc
				r.processFunc = r.Emit
				if _, err = r.file.Seek(r.Offset, 0); err != nil {
					r.Errorw("Failed to seek post-header", zap.Error(err))
					return
				}
				s = scanner.New(r, r.MaxLogSize, scanner.DefaultBufferSize, r.Offset, r.splitFunc)
			} else {
				r.Errorw("process: %w", zap.Error(err))
			}
		}

		r.Offset = s.Pos()
	}
}

func (r *Reader) finalizeHeader() {
	if err := r.headerReader.Stop(); err != nil {
		r.Errorw("Failed to stop header pipeline during finalization", zap.Error(err))
	}
	r.headerReader = nil
	r.HeaderFinalized = true
}

// Delete will close and delete the file
func (r *Reader) Delete() {
	if r.file == nil {
		return
	}
	r.Close()
	if err := os.Remove(r.FileName); err != nil {
		r.Errorf("could not delete %s", r.FileName)
	}
}

// Close will close the file
func (r *Reader) Close() {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.Debugw("Problem closing reader", zap.Error(err))
		}
		r.file = nil
	}

	if r.headerReader != nil {
		if err := r.headerReader.Stop(); err != nil {
			r.Errorw("Failed to stop header pipeline", zap.Error(err))
		}
	}
}

// Read from the file and update the fingerprint if necessary
func (r *Reader) Read(dst []byte) (int, error) {
	// Skip if fingerprint is already built
	// or if fingerprint is behind Offset
	if len(r.Fingerprint.FirstBytes) == r.FingerprintSize || int(r.Offset) > len(r.Fingerprint.FirstBytes) {
		return r.file.Read(dst)
	}
	n, err := r.file.Read(dst)
	appendCount := min0(n, r.FingerprintSize-int(r.Offset))
	// return for n == 0 or r.Offset >= r.FingerprintSize
	if appendCount == 0 {
		return n, err
	}

	// for appendCount==0, the following code would add `0` to fingerprint
	r.Fingerprint.FirstBytes = append(r.Fingerprint.FirstBytes[:r.Offset], dst[:appendCount]...)
	return n, err
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

// validateFingerprint checks whether or not the reader still has a valid file handle.
//
// It creates a new fingerprint from the old file handle and compares it to the
// previously known fingerprint. If there has been a change to the fingerprint
// (other than appended data), the file is considered invalid. Consequently, the
// reader will automatically close the file and drop the handle.
//
// The function returns true if the file handle is still valid, false otherwise.
func (r *Reader) ValidateFingerprint() bool {
	if r.file == nil {
		return false
	}
	refreshedFingerprint, err := fingerprint.New(r.file, r.FingerprintSize)
	if err != nil {
		r.Debugw("Failed to create fingerprint", zap.Error(err))
		return false
	}
	return refreshedFingerprint.StartsWith(r.Fingerprint)
}
