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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
)

type Metadata struct {
	Fingerprint     *fingerprint.Fingerprint
	Offset          int64
	FileAttributes  map[string]any
	HeaderFinalized bool
	FlushState      *flush.State
}

// Reader manages a single file
type Reader struct {
	*Metadata
	logger          *zap.SugaredLogger
	fileName        string
	file            *os.File
	fingerprintSize int
	maxLogSize      int
	lineSplitFunc   bufio.SplitFunc
	splitFunc       bufio.SplitFunc
	decoder         *decode.Decoder
	headerReader    *header.Reader
	processFunc     emit.Callback
	emitFunc        emit.Callback
	deleteAtEOF     bool
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
	return fingerprint.New(r.file, r.fingerprintSize)
}

// ReadToEnd will read until the end of the file
func (r *Reader) ReadToEnd(ctx context.Context) {
	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.logger.Errorw("Failed to seek", zap.Error(err))
		return
	}

	s := scanner.New(r, r.maxLogSize, scanner.DefaultBufferSize, r.Offset, r.splitFunc)

	// Iterate over the tokenized file, emitting entries as we go
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ok := s.Scan()
		if !ok {
			if err := s.Error(); err != nil {
				r.logger.Errorw("Failed during scan", zap.Error(err))
			} else if r.deleteAtEOF {
				r.delete()
			}
			break
		}

		token, err := r.decoder.Decode(s.Bytes())
		if err != nil {
			r.logger.Errorw("decode: %w", zap.Error(err))
		} else if err := r.processFunc(ctx, token, r.FileAttributes); err != nil {
			if errors.Is(err, header.ErrEndOfHeader) {
				r.finalizeHeader()

				// Now that the header is consumed, use the normal split and process functions.
				// Recreate the scanner with the normal split func.
				// Do not use the updated offset from the old scanner, as the most recent token
				// could be split differently with the new splitter.
				r.splitFunc = r.lineSplitFunc
				r.processFunc = r.emitFunc
				if _, err = r.file.Seek(r.Offset, 0); err != nil {
					r.logger.Errorw("Failed to seek post-header", zap.Error(err))
					return
				}
				s = scanner.New(r, r.maxLogSize, scanner.DefaultBufferSize, r.Offset, r.splitFunc)
			} else {
				r.logger.Errorw("process: %w", zap.Error(err))
			}
		}
		r.Offset = s.Pos()
	}
}

func (r *Reader) finalizeHeader() {
	if err := r.headerReader.Stop(); err != nil {
		r.logger.Errorw("Failed to stop header pipeline during finalization", zap.Error(err))
	}
	r.headerReader = nil
	r.HeaderFinalized = true
}

// Delete will close and delete the file
func (r *Reader) delete() {
	r.Close()
	if err := os.Remove(r.fileName); err != nil {
		r.logger.Errorf("could not delete %s", r.fileName)
	}
}

// Close will close the file and return the metadata
func (r *Reader) Close() *Metadata {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.logger.Debugw("Problem closing reader", zap.Error(err))
		}
		r.file = nil
	}

	if r.headerReader != nil {
		if err := r.headerReader.Stop(); err != nil {
			r.logger.Errorw("Failed to stop header pipeline", zap.Error(err))
		}
	}
	m := r.Metadata
	r.Metadata = nil
	return m
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
	// return for n == 0 or r.Offset >= r.fingerprintSize
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

func (r *Reader) NameEquals(other *Reader) bool {
	return r.fileName == other.fileName
}

// Validate returns true if the reader still has a valid file handle, false otherwise.
func (r *Reader) Validate() bool {
	if r.file == nil {
		return false
	}
	refreshedFingerprint, err := fingerprint.New(r.file, r.fingerprintSize)
	if err != nil {
		return false
	}
	if refreshedFingerprint.StartsWith(r.Fingerprint) {
		return true
	}
	return false
}
