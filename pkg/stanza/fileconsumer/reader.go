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
	emit            EmitFunc
}

// Reader manages a single file
type Reader struct {
	*zap.SugaredLogger `json:"-"` // json tag excludes embedded fields from storage
	*readerConfig
	lineSplitFunc bufio.SplitFunc
	splitFunc     bufio.SplitFunc
	encoding      helper.Encoding
	processFunc   EmitFunc

	Fingerprint    *Fingerprint
	bufferSize     int
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

	scanner := NewPositionalScanner(r, r.maxLogSize, r.Offset, r.bufferSize, r.splitFunc)

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

			scanner = NewPositionalScanner(r, r.maxLogSize, r.Offset, r.bufferSize, r.splitFunc)
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

func min0(a, b int) int {
	if a < 0 || b < 0 {
		return 0
	}
	if a < b {
		return a
	}
	return b
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
