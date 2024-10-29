// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
)

type Metadata struct {
	Fingerprint     *fingerprint.Fingerprint
	Offset          int64
	RecordNum       int64
	FileAttributes  map[string]any
	HeaderFinalized bool
	FlushState      *flush.State
}

// Reader manages a single file
type Reader struct {
	*Metadata
	set                    component.TelemetrySettings
	fileName               string
	file                   *os.File
	reader                 io.Reader
	fingerprintSize        int
	initialBufferSize      int
	maxLogSize             int
	lineSplitFunc          bufio.SplitFunc
	splitFunc              bufio.SplitFunc
	decoder                *decode.Decoder
	headerReader           *header.Reader
	processFunc            emit.Callback
	emitFunc               emit.Callback
	deleteAtEOF            bool
	needsUpdateFingerprint bool
	includeFileRecordNum   bool
	compression            string
	acquireFSLock          bool
}

// ReadToEnd will read until the end of the file
func (r *Reader) ReadToEnd(ctx context.Context) {
	if r.acquireFSLock {
		if !r.tryLockFile() {
			return
		}
		defer r.unlockFile()
	}

	switch r.compression {
	case "gzip":
		// We need to create a gzip reader each time ReadToEnd is called because the underlying
		// SectionReader can only read a fixed window (from previous offset to EOF).
		info, err := r.file.Stat()
		if err != nil {
			r.set.Logger.Error("Failed to stat", zap.Error(err))
			return
		}
		currentEOF := info.Size()

		// use a gzip Reader with an underlying SectionReader to pick up at the last
		// offset of a gzip compressed file
		gzipReader, err := gzip.NewReader(io.NewSectionReader(r.file, r.Offset, currentEOF))
		if err != nil {
			if !errors.Is(err, io.EOF) {
				r.set.Logger.Error("Failed to create gzip reader", zap.Error(err))
			}
			return
		} else {
			r.reader = gzipReader
		}
		// Offset tracking in an uncompressed file is based on the length of emitted tokens, but in this case
		// we need to set the offset to the end of the file.
		defer func() {
			r.Offset = currentEOF
		}()
	default:
		r.reader = r.file
	}

	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.set.Logger.Error("Failed to seek", zap.Error(err))
		return
	}

	defer func() {
		if r.needsUpdateFingerprint {
			r.updateFingerprint()
		}
	}()

	s := scanner.New(r, r.maxLogSize, r.initialBufferSize, r.Offset, r.splitFunc)

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
				r.set.Logger.Error("Failed during scan", zap.Error(err))
			} else if r.deleteAtEOF {
				r.delete()
			}
			return
		}

		token, err := r.decoder.Decode(s.Bytes())
		if err != nil {
			r.set.Logger.Error("decode: %w", zap.Error(err))
			r.Offset = s.Pos() // move past the bad token or we may be stuck
			continue
		}

		if r.includeFileRecordNum {
			r.RecordNum++
			r.FileAttributes[attrs.LogFileRecordNumber] = r.RecordNum
		}

		err = r.processFunc(ctx, token, r.FileAttributes)
		if err == nil {
			r.Offset = s.Pos() // successful emit, update offset
			continue
		}

		if !errors.Is(err, header.ErrEndOfHeader) {
			r.set.Logger.Error("process: %w", zap.Error(err))
			r.Offset = s.Pos() // move past the bad token or we may be stuck
			continue
		}

		// Clean up the header machinery
		if err = r.headerReader.Stop(); err != nil {
			r.set.Logger.Error("Failed to stop header pipeline during finalization", zap.Error(err))
		}
		r.headerReader = nil
		r.HeaderFinalized = true

		// Switch to the normal split and process functions.
		r.splitFunc = r.lineSplitFunc
		r.processFunc = r.emitFunc

		// Recreate the scanner with the normal split func.
		// Do not use the updated offset from the old scanner, as the most recent token
		// could be split differently with the new splitter.
		if _, err = r.file.Seek(r.Offset, 0); err != nil {
			r.set.Logger.Error("Failed to seek post-header", zap.Error(err))
			return
		}
		s = scanner.New(r, r.maxLogSize, scanner.DefaultBufferSize, r.Offset, r.splitFunc)
	}
}

// Delete will close and delete the file
func (r *Reader) delete() {
	r.close()
	if err := os.Remove(r.fileName); err != nil {
		r.set.Logger.Error("could not delete", zap.String("filename", r.fileName))
	}
}

// Close will close the file and return the metadata
func (r *Reader) Close() *Metadata {
	r.close()
	m := r.Metadata
	r.Metadata = nil
	return m
}

func (r *Reader) close() {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.set.Logger.Debug("Problem closing reader", zap.Error(err))
		}
		r.file = nil
	}

	if r.headerReader != nil {
		if err := r.headerReader.Stop(); err != nil {
			r.set.Logger.Error("Failed to stop header pipeline", zap.Error(err))
		}
	}
}

// Read from the file and update the fingerprint if necessary
func (r *Reader) Read(dst []byte) (n int, err error) {
	n, err = r.reader.Read(dst)
	if n == 0 || err != nil {
		return
	}

	if !r.needsUpdateFingerprint && r.Fingerprint.Len() < r.fingerprintSize {
		r.needsUpdateFingerprint = true
	}
	return
}

func (r *Reader) NameEquals(other *Reader) bool {
	return r.fileName == other.fileName
}

// Validate returns true if the reader still has a valid file handle, false otherwise.
func (r *Reader) Validate() bool {
	if r.file == nil {
		return false
	}
	refreshedFingerprint, err := fingerprint.NewFromFile(r.file, r.fingerprintSize)
	if err != nil {
		return false
	}
	if refreshedFingerprint.StartsWith(r.Fingerprint) {
		return true
	}
	return false
}

func (r *Reader) GetFileName() string {
	return r.fileName
}

func (m Metadata) GetFingerprint() *fingerprint.Fingerprint {
	return m.Fingerprint
}

func (r *Reader) updateFingerprint() {
	r.needsUpdateFingerprint = false
	if r.file == nil {
		return
	}
	refreshedFingerprint, err := fingerprint.NewFromFile(r.file, r.fingerprintSize)
	if err != nil {
		return
	}
	if r.Fingerprint.Len() > 0 && !refreshedFingerprint.StartsWith(r.Fingerprint) {
		return // fingerprint tampered, likely due to truncation
	}
	r.Fingerprint = refreshedFingerprint
}
