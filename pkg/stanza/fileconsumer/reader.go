// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decoder"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
)

type readerConfig struct {
	fingerprintSize         int
	maxLogSize              int
	emit                    emit.Callback
	includeFileName         bool
	includeFilePath         bool
	includeFileNameResolved bool
	includeFilePathResolved bool
}

type readerMetadata struct {
	Fingerprint     *fingerprint.Fingerprint
	Offset          int64
	FileAttributes  map[string]any
	HeaderFinalized bool
}

// reader manages a single file
type reader struct {
	*zap.SugaredLogger
	*readerConfig
	*readerMetadata
	file          *os.File
	lineSplitFunc bufio.SplitFunc
	splitFunc     bufio.SplitFunc
	decoder       *decoder.Decoder
	headerReader  *header.Reader
	processFunc   emit.Callback
	eof           bool
}

// offsetToEnd sets the starting offset
func (r *reader) offsetToEnd() error {
	info, err := r.file.Stat()
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}
	r.Offset = info.Size()
	return nil
}

// ReadToEnd will read until the end of the file
func (r *reader) ReadToEnd(ctx context.Context) {
	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.Errorw("Failed to seek", zap.Error(err))
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
			r.eof = true
			if err := s.Error(); err != nil {
				// If Scan returned an error then we are not guaranteed to be at the end of the file
				r.eof = false
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
				r.processFunc = r.emit
				if _, err = r.file.Seek(r.Offset, 0); err != nil {
					r.Errorw("Failed to seek post-header", zap.Error(err))
					return
				}
				s = scanner.New(r, r.maxLogSize, scanner.DefaultBufferSize, r.Offset, r.splitFunc)
			} else {
				r.Errorw("process: %w", zap.Error(err))
			}
		}

		r.Offset = s.Pos()
	}
}

func (r *reader) finalizeHeader() {
	if err := r.headerReader.Stop(); err != nil {
		r.Errorw("Failed to stop header pipeline during finalization", zap.Error(err))
	}
	r.headerReader = nil
	r.HeaderFinalized = true
}

// Close will close the file
func (r *reader) Close() {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.Debugw("Problem closing reader", zap.Error(err))
		}
	}

	if r.headerReader != nil {
		if err := r.headerReader.Stop(); err != nil {
			r.Errorw("Failed to stop header pipeline", zap.Error(err))
		}
	}
}

// Read from the file and update the fingerprint if necessary
func (r *reader) Read(dst []byte) (int, error) {
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

func min0(a, b int) int {
	if a < 0 || b < 0 {
		return 0
	}
	if a < b {
		return a
	}
	return b
}
