package reader

import (
	"bytes"
	"compress/gzip"
	"errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"math"
	"os"
)

type CompressedReader struct {
	*Reader

	buffer           *bytes.Buffer
	lastReadFileSize int64
	fromBeginning    bool
}

func (r *CompressedReader) ReadToEnd(ctx context.Context) {
	newFileHandle, err := os.Open(r.file.Name())
	if err != nil {
		r.set.Logger.Error("problem creating reader for compressed file", zap.Error(err))
		return
	}
	r.file = newFileHandle
	gzipReader, err := gzip.NewReader(r.file)
	if err != nil {
		r.set.Logger.Error("problem creating reader for compressed file", zap.Error(err))
		return
	}

	// TODO validate if we can keep this check for the file size to avoid unnecessary decompression
	// in theory, this check can lead to new log entries going unnoticed, if the new entries are duplicates
	// of a previous entry, as this leads to the size of the compressed file not being increased.
	// practically this may not be an issue for logs, since at least the timestamp should change for a new log entry,
	// even if it has the same message as the line before

	stat, err := r.file.Stat()
	if err != nil {
		r.set.Logger.Error("problem creating determining size of compressed file", zap.String("fileName", r.fileName), zap.Error(err))
		return
	}

	initalRead := true
	if lastReadFileSize, ok := r.FileAttributes["lastReadFileSize"]; ok {
		initalRead = false
		if lastReadFileSize.(int64) >= stat.Size() {
			// do not read further unless something has been added to the file
			return
		}
	}

	r.FileAttributes["lastReadFileSize"] = stat.Size()

	uncompressedBytes := make([]byte, r.maxLogSize)
	nrBytes, err := gzipReader.Read(uncompressedBytes)

	scannedBytes := int64(0)
	defer func() {
		if r.needsUpdateFingerprint {
			r.updateFingerprint()
		}
		if !r.fromBeginning && initalRead {
			r.Offset = int64(nrBytes)
		} else {
			r.Offset = int64(math.Min(float64(nrBytes), float64(r.Offset+scannedBytes)))
		}
	}()

	if !r.fromBeginning && initalRead {
		// if we don't want to read through the file from the beginning,
		// only remember the read file size and process changes from there on
		return
	}

	if int64(nrBytes) < r.Offset {
		// avoid index out of range error if something got deleted
		return
	}

	r.buffer = bytes.NewBuffer(uncompressedBytes[r.Offset:nrBytes])

	s := scanner.New(r.buffer, r.maxLogSize, r.initialBufferSize, 0, r.splitFunc)

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
			scannedBytes = s.Pos() // move past the bad token or we may be stuck
			continue
		}

		err = r.processFunc(ctx, token, r.FileAttributes)
		if err == nil {
			scannedBytes = s.Pos() // successful emit, update offset
			continue
		}

		if !errors.Is(err, header.ErrEndOfHeader) {
			r.set.Logger.Error("process: %w", zap.Error(err))
			scannedBytes = s.Pos() // move past the bad token or we may be stuck
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
