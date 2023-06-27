// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type readerConfig struct {
	fingerprintSize int
	maxLogSize      int
	bufferSize      int
	emit            EmitFunc
}

// Reader manages a single file
//
// Deprecated: [v0.80.0] This will be made internal in a future release, tentatively v0.82.0.
type Reader struct {
	*zap.SugaredLogger `json:"-"` // json tag excludes embedded fields from storage
	*readerConfig
	lineSplitFunc bufio.SplitFunc
	splitFunc     bufio.SplitFunc
	encoding      helper.Encoding
	processFunc   EmitFunc

	Fingerprint    *Fingerprint
	Offset         int64
	generation     int
	file           *os.File
	FileAttributes *FileAttributes
	eof            bool

	HeaderFinalized bool
	recreateScanner bool

	headerSettings       *headerSettings
	headerPipeline       pipeline.Pipeline
	headerPipelineOutput *headerPipelineOutput
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
	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.Errorw("Failed to seek", zap.Error(err))
		return
	}

	bufferSize := r.bufferSize
	if r.bufferSize < r.fingerprintSize {
		bufferSize = r.fingerprintSize
	}
	scanner := NewPositionalScanner(r, r.maxLogSize, bufferSize, r.Offset, r.splitFunc)

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
			r.processFunc(ctx, r.FileAttributes, token)
		}

		if r.recreateScanner {
			r.recreateScanner = false
			// recreate the scanner with the log-line's split func.
			// We do not use the updated offset from the scanner,
			// as the log line we just read could be multiline, and would be
			// split differently with the new splitter.
			if _, err := r.file.Seek(r.Offset, 0); err != nil {
				r.Errorw("Failed to seek post-header", zap.Error(err))
				return
			}

			scanner = NewPositionalScanner(r, r.maxLogSize, r.bufferSize, r.Offset, r.splitFunc)
		}

		r.Offset = scanner.Pos()
	}
}

// consumeHeaderLine checks if the given token is a line of the header, and consumes it if it is.
// The return value dictates whether the given line was a header line or not.
// If false is returned, the full header can be assumed to be read.
func (r *Reader) consumeHeaderLine(ctx context.Context, _ *FileAttributes, token []byte) {
	if !r.headerSettings.matchRegex.Match(token) {
		// Finalize and cleanup the pipeline
		r.HeaderFinalized = true

		// Stop and drop the header pipeline.
		if err := r.headerPipeline.Stop(); err != nil {
			r.Errorw("Failed to stop header pipeline during finalization", zap.Error(err))
		}
		r.headerPipeline = nil
		r.headerPipelineOutput = nil

		// Use the line split func instead of the header split func
		r.splitFunc = r.lineSplitFunc
		r.processFunc = r.emit
		// Mark that we should recreate the scanner, since we changed the split function
		r.recreateScanner = true
		return
	}

	firstOperator := r.headerPipeline.Operators()[0]

	newEntry := entry.New()
	newEntry.Body = string(token)

	if err := firstOperator.Process(ctx, newEntry); err != nil {
		r.Errorw("Failed to process header entry", zap.Error(err))
		return
	}

	ent, err := r.headerPipelineOutput.WaitForEntry(ctx)
	if err != nil {
		r.Errorw("Error while waiting for header entry", zap.Error(err))
		return
	}

	// Copy resultant attributes over current set of attributes (upsert)
	for k, v := range ent.Attributes {
		r.FileAttributes.HeaderAttributes[k] = v
	}
}

// Close will close the file
func (r *Reader) Close() {
	if r.file != nil {
		if err := r.file.Close(); err != nil {
			r.Debugw("Problem closing reader", zap.Error(err))
		}
	}

	if r.headerPipeline != nil {
		if err := r.headerPipeline.Stop(); err != nil {
			r.Errorw("Failed to stop header pipeline", zap.Error(err))
		}
	}
}

// Read from the file and update the fingerprint if necessary
func (r *Reader) Read(dst []byte) (n int, err error) {
	n, err = r.file.Read(dst)

	if len(r.Fingerprint.FirstBytes) == r.fingerprintSize {
		// Steady state. Just return data to scanner.
		return
	}

	if len(r.Fingerprint.FirstBytes) > r.fingerprintSize {
		// Oversized fingerprint. The component was restarted with a decreased 'fingerprint_size'.
		// Just return data to scanner.
		return
	}

	if int(r.Offset) > len(r.Fingerprint.FirstBytes) {
		// Undersized fingerprint.  The component was restarted with an increased 'fingerprint_size.
		// However, we've already read past the fingerprint. Just keep reading.
		return
	}

	if len(r.Fingerprint.FirstBytes) == int(r.Offset) {
		// The fingerprint is incomplete but is exactly aligned with the offset.
		// Take advantage of the simple case and avoid some computation.
		appendCount := r.fingerprintSize - len(r.Fingerprint.FirstBytes)
		if appendCount > n {
			appendCount = n
		}
		r.Fingerprint.FirstBytes = append(r.Fingerprint.FirstBytes, dst[:appendCount]...)
	}

	// The fingerprint is incomplete and is NOT aligned with the offset. This means the fingerprint
	// contains data that hasn't yet been emitted. Either we observed an incomplete token at the end of the
	// file, or we are running with 'start_at: beginning' in which case the fingerprint is initialized
	// independently of the Reader.

	// Allowing the fingerprint to run ahead of tokenization improves our ability to uniquely identify files.
	// However, it also means we must compensate for the misalignment when appending to the fingerprint.

	// WE MUST ASSUME that the fingerprint will never contain a token longer than the 'dst' buffer.
	// The easiest way to enforce this is to ensure the buffer is at least as large as the fingerprint.
	// Unfortunately, this must be enforced outside of this function.
	// Without this guarantee, the scanner may call this function consecutively before we are able to update
	// the offset, which means we cannot trust the offset to tell us which data in the 'dst' buffer has
	// already been appended to the fingerprint.

	newBytesIndex := len(r.Fingerprint.FirstBytes) - int(r.Offset)
	if n <= newBytesIndex {
		// Already have this data in the fingerprint. Just return data to scanner.
		return
	}

	appendCount := r.fingerprintSize - len(r.Fingerprint.FirstBytes)
	if appendCount > n-newBytesIndex {
		// Not enough new data to complete the fingerprint, but append what we have.
		appendCount = n - newBytesIndex
	}
	r.Fingerprint.FirstBytes = append(r.Fingerprint.FirstBytes, dst[newBytesIndex:newBytesIndex+appendCount]...)
	return
}

// mapCopy deep copies the provided attributes map.
func mapCopy(m map[string]any) map[string]any {
	newMap := make(map[string]any, len(m))
	for k, v := range m {
		switch typedVal := v.(type) {
		case map[string]any:
			newMap[k] = mapCopy(typedVal)
		default:
			// Assume any other values are safe to directly copy.
			// Struct types and slice types shouldn't appear in attribute maps from pipelines
			newMap[k] = v
		}
	}
	return newMap
}
