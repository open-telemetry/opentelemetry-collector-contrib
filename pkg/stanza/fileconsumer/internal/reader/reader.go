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
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenlen"
)

const gzipExtension = ".gz"

type Metadata struct {
	Fingerprint      *fingerprint.Fingerprint
	Offset           int64
	RecordNum        int64
	FileAttributes   map[string]any
	HeaderFinalized  bool
	FlushState       flush.State
	TokenLenState    tokenlen.State
	FileType         string
	TruncateSkipping bool

	// LastObservedPath and LastObservedMtime are used by the
	// skip_unmodified_files config option to skip re-opening and
	// re-fingerprinting a file whose path+mtime is unchanged since the last
	// observation. They are populated on a best-effort basis by the manager
	// when the option is enabled and are safe to leave at their zero values
	// (the option-off code path does not read them).
	LastObservedPath  string
	LastObservedMtime time.Time

	// DecompressedBytesToSkip is the number of decompressed bytes at Offset in a gzip compressed file that were
	// already read. Offset can only point to the start of a gzip member, so this is set when a gzip member was
	// read while it was still being written, and when a partially read plaintext file was compressed.
	DecompressedBytesToSkip int64

	// incompleteGzipSize is the size of a gzip compressed file when its last gzip member was last found to be
	// incomplete, and incompleteGzipSince is when the file stopped growing. They are used to avoid decompressing
	// the file again while it has not grown, and to warn about a gzip member that stays incomplete.
	// They are not persisted.
	incompleteGzipSize   int64
	incompleteGzipSince  time.Time
	incompleteGzipWarned bool
}

// incompleteGzipWarnAfter is how long a gzip member can stay incomplete, without its file growing,
// before a warning is logged.
const incompleteGzipWarnAfter = time.Minute

// Reader manages a single file
type Reader struct {
	*Metadata
	set                    component.TelemetrySettings
	fileName               string
	file                   *os.File
	reader                 io.Reader
	fingerprintSize        int
	bufPool                *sync.Pool
	initialBufferSize      int
	maxLogSize             int
	headerSplitFunc        bufio.SplitFunc
	contentSplitFunc       bufio.SplitFunc
	decoder                *encoding.Decoder
	headerReader           *header.Reader
	emitFunc               emit.Callback
	deleteAtEOF            bool
	needsUpdateFingerprint bool
	compression            string
	acquireFSLock          bool
	fileCacheAdvise        bool
	maxBatchSize           int
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
		gzipReader, err := r.createGzipReader()
		if err != nil {
			return
		}
		defer r.updateGzipOffset(gzipReader)
	case "auto":
		if r.FileType == gzipExtension {
			gzipReader, err := r.createGzipReader()
			if err != nil {
				return
			}
			defer r.updateGzipOffset(gzipReader)
		} else {
			r.reader = r.file
		}
	default:
		r.reader = r.file
	}

	if r.fileCacheAdvise && r.FileType != gzipExtension {
		r.fadviseFile()
	}

	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.set.Logger.Error("failed to seek", zap.Error(err))
		return
	}

	defer func() {
		if r.needsUpdateFingerprint {
			r.updateFingerprint()
		}
	}()

	if r.headerReader != nil {
		if r.readHeader(ctx) {
			return
		}
	}

	r.readContents(ctx)
}

// createGzipReader creates a reader that decompresses the gzip members of the file from the offset to the end of the file.
func (r *Reader) createGzipReader() (*gzipMembersReader, error) {
	// We need to create a gzip reader each time ReadToEnd is called because the underlying
	// SectionReader can only read a fixed window (from previous offset to EOF).
	info, err := r.file.Stat()
	if err != nil {
		r.set.Logger.Error("failed to stat", zap.Error(err))
		return nil, err
	}
	currentEOF := info.Size()

	if r.incompleteGzipSize > 0 && r.incompleteGzipSize == currentEOF {
		// The file has not grown since its last gzip member was found to be incomplete, so there is nothing new to read.
		r.handleUnchangedIncompleteGzip()
		return nil, io.ErrUnexpectedEOF
	}

	gzipReader, err := newGzipMembersReader(io.NewSectionReader(r.file, r.Offset, currentEOF-r.Offset), r.Offset, currentEOF)
	if err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			r.setIncompleteGzip(currentEOF)
		} else if !errors.Is(err, io.EOF) {
			r.set.Logger.Error("failed to create gzip reader", zap.Error(err),
				zap.Int64("offset", r.Offset), zap.Int64("file_size", currentEOF))
		}
		return nil, err
	}

	// Skip past already-consumed decompressed bytes so only new lines are processed.
	if r.DecompressedBytesToSkip > 0 {
		if _, err := io.CopyN(io.Discard, gzipReader, r.DecompressedBytesToSkip); err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				r.setIncompleteGzip(currentEOF)
			} else {
				r.set.Logger.Error("failed to skip already-consumed decompressed bytes", zap.Error(err),
					zap.Int64("offset", r.Offset), zap.Int64("decompressed_bytes_to_skip", r.DecompressedBytesToSkip),
					zap.Int64("file_size", currentEOF))
			}
			return nil, err
		}
	}
	r.reader = gzipReader
	return gzipReader, nil
}

// updateGzipOffset moves the offset of a gzip compressed file past the data that was read. The offset can only
// point to the start of a gzip member, so if the data that was read ends inside a gzip member, for example
// because the member is still being written, the offset points to the start of that member and the
// decompressed bytes of it that were already read are skipped the next time.
func (r *Reader) updateGzipOffset(g *gzipMembersReader) {
	if g.err != nil && !errors.Is(g.err, io.ErrUnexpectedEOF) {
		// The file was read to its end, or as far as it could be decompressed.
		if !errors.Is(g.err, io.EOF) {
			validEnd := g.start
			if len(g.ends) > 0 {
				validEnd += g.ends[len(g.ends)-1].compressed
			}
			r.set.Logger.Error("invalid gzip data, the rest of the file after the last complete gzip member is skipped",
				zap.Error(g.err), zap.Int64("last_complete_gzip_member_end", validEnd),
				zap.Int64("skipped_bytes", g.eof-validEnd), zap.Int64("file_size", g.eof))
		}
		r.Offset, r.DecompressedBytesToSkip = g.eof, 0
		r.incompleteGzipSize, r.incompleteGzipSince, r.incompleteGzipWarned = 0, time.Time{}, false
		return
	}

	// While reading, Offset was moved by the length of the decompressed data that was read.
	read := r.DecompressedBytesToSkip + r.Offset - g.start
	offset, skip := g.start, read
	for _, end := range g.ends {
		if end.decompressed > read {
			break
		}
		offset, skip = g.start+end.compressed, read-end.decompressed
	}
	r.Offset, r.DecompressedBytesToSkip = offset, skip
	if errors.Is(g.err, io.ErrUnexpectedEOF) {
		r.setIncompleteGzip(g.eof)
	}
}

func (r *Reader) setIncompleteGzip(size int64) {
	if r.incompleteGzipSize != size {
		r.incompleteGzipSize, r.incompleteGzipSince, r.incompleteGzipWarned = size, time.Now(), false
	}
}

func (r *Reader) handleUnchangedIncompleteGzip() {
	unchangedFor := time.Since(r.incompleteGzipSince)
	if !r.incompleteGzipWarned && unchangedFor >= incompleteGzipWarnAfter {
		r.set.Logger.Warn("gzip member is incomplete and the file has not grown, the rest of it will be read once it is written",
			zap.Duration("unchanged_for", unchangedFor), zap.Int64("offset", r.Offset),
			zap.Int64("decompressed_bytes_read", r.DecompressedBytesToSkip), zap.Int64("file_size", r.incompleteGzipSize))
		r.incompleteGzipWarned = true
	}
}

// gzipMembersReader decompresses the gzip members read from a section of a file one after another, like a
// multistream gzip reader, and records where each complete member ends in the file and in the decompressed data.
// If the section ends inside a gzip member, which happens while it is still being written, Read returns
// io.ErrUnexpectedEOF after the data that could be decompressed.
type gzipMembersReader struct {
	cr           *countingByteReader
	gr           *gzip.Reader
	start, eof   int64
	decompressed int64
	ends         []gzipMemberEnd
	err          error
}

type gzipMemberEnd struct {
	compressed   int64
	decompressed int64
}

func newGzipMembersReader(rd io.Reader, start, eof int64) (*gzipMembersReader, error) {
	cr := &countingByteReader{r: bufio.NewReader(rd)}
	gr, err := gzip.NewReader(cr)
	if err != nil {
		return nil, err
	}
	gr.Multistream(false)
	return &gzipMembersReader{cr: cr, gr: gr, start: start, eof: eof}, nil
}

func (g *gzipMembersReader) Read(p []byte) (int, error) {
	if g.err != nil {
		return 0, g.err
	}
	for {
		n, err := g.gr.Read(p)
		g.decompressed += int64(n)
		if errors.Is(err, io.EOF) {
			g.ends = append(g.ends, gzipMemberEnd{compressed: g.cr.n, decompressed: g.decompressed})
			// Reset returns io.EOF if no other gzip member follows.
			if err = g.gr.Reset(g.cr); err == nil {
				g.gr.Multistream(false)
				if n == 0 {
					continue
				}
			}
		}
		g.err = err
		return n, err
	}
}

// countingByteReader counts the bytes read through it. It implements io.ByteReader, so a gzip reader reading
// from it does not read past the end of a gzip member.
type countingByteReader struct {
	r *bufio.Reader
	n int64
}

func (c *countingByteReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func (c *countingByteReader) ReadByte() (byte, error) {
	b, err := c.r.ReadByte()
	if err == nil {
		c.n++
	}
	return b, err
}

func (r *Reader) readHeader(ctx context.Context) (doneReadingFile bool) {
	bufPtr := r.getBufPtrFromPool()
	defer r.bufPool.Put(bufPtr)
	s := scanner.New(r, r.maxLogSize, *bufPtr, r.Offset, r.headerSplitFunc, r.FileType == gzipExtension)

	// Read the tokens from the file until no more header tokens are found or the end of file is reached.
	for {
		select {
		case <-ctx.Done():
			return true
		default:
		}

		ok := s.Scan()
		if !ok {
			if err := s.Error(); errors.Is(err, io.ErrUnexpectedEOF) {
				r.set.Logger.Debug("gzip member is not complete yet, the rest of it will be read once it is written")
			} else if err != nil {
				r.set.Logger.Error("failed during header scan", zap.Error(err))
			} else {
				r.set.Logger.Debug("end of file reached", zap.Bool("delete_at_eof", r.deleteAtEOF))
				if r.deleteAtEOF {
					r.delete()
				}
			}
			// Either end of file was reached, or file cannot be scanned.
			return true
		}

		token, err := textutils.DecodeAsString(r.decoder, s.Bytes())
		if err != nil {
			r.set.Logger.Error("failed to decode header token", zap.Error(err))
			r.Offset = s.Pos() // move past the bad token or we may be stuck
			continue
		}

		err = r.headerReader.Process(ctx, token, r.FileAttributes)
		if err != nil {
			if errors.Is(err, header.ErrEndOfHeader) {
				// End of header reached.
				break
			}
			r.set.Logger.Error("failed to process header token", zap.Error(err))
		}

		r.Offset = s.Pos()
	}

	// Clean up the header machinery
	if err := r.headerReader.Stop(); err != nil {
		r.set.Logger.Error("failed to stop header pipeline during finalization", zap.Error(err))
	}
	r.headerReader = nil
	r.HeaderFinalized = true

	// Reset position in file to r.Offset after the header scanner might have moved it past a content token.
	if _, err := r.file.Seek(r.Offset, 0); err != nil {
		r.set.Logger.Error("failed to seek post-header", zap.Error(err))
		return true
	}

	return false
}

func (r *Reader) readContents(ctx context.Context) {
	var buf []byte
	if r.TokenLenState.MinimumLength <= r.initialBufferSize {
		bufPtr := r.getBufPtrFromPool()
		buf = *bufPtr
		defer r.bufPool.Put(bufPtr)
	} else {
		// If we previously saw a potential token larger than the default buffer,
		// size the buffer to be at least one byte larger so we can see if there's more data.
		// Usually, expect this to be a rare event so that we don't bother pooling this special buffer size.
		buf = make([]byte, 0, r.TokenLenState.MinimumLength+1)
	}
	s := scanner.New(r, r.maxLogSize, buf, r.Offset, r.contentSplitFunc, r.FileType == gzipExtension)

	tokenBodies := make([][]byte, r.maxBatchSize)
	tokenOffsets := make([]int64, r.maxBatchSize+1)

	numTokensBatched := 0
	tokenOffsets[0] = r.Offset
	// Iterate over the contents of the file.
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ok := s.Scan()
		if !ok {
			if err := s.Error(); errors.Is(err, io.ErrUnexpectedEOF) {
				r.set.Logger.Debug("gzip member is not complete yet, the rest of it will be read once it is written")
			} else if err != nil {
				r.set.Logger.Error("failed during scan", zap.Error(err))
			} else if r.deleteAtEOF {
				r.delete()
			}

			if numTokensBatched > 0 {
				err := r.emitFunc(ctx, tokenBodies[:numTokensBatched], r.FileAttributes, r.RecordNum, tokenOffsets)
				if err != nil {
					r.set.Logger.Error("failed to emit token", zap.Error(err))
				}
				r.Offset = s.Pos()
			}
			return
		}

		var err error
		tokenBodies[numTokensBatched], err = r.decoder.Bytes(s.Bytes())
		tokenOffsets[numTokensBatched+1] = s.Pos()
		if err != nil {
			r.set.Logger.Error("failed to decode token", zap.Error(err))
			r.Offset = s.Pos() // move past the bad token or we may be stuck
			continue
		}
		numTokensBatched++

		r.RecordNum++
		if r.maxBatchSize > 0 && numTokensBatched >= r.maxBatchSize {
			if err = r.emitFunc(ctx, tokenBodies[:numTokensBatched], r.FileAttributes, r.RecordNum, tokenOffsets); err != nil {
				r.set.Logger.Error("failed to emit token", zap.Error(err))
			}
			numTokensBatched = 0
			r.Offset, tokenOffsets[0] = s.Pos(), s.Pos()
		}
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
		return n, err
	}

	if !r.needsUpdateFingerprint && r.Fingerprint.Len() < r.fingerprintSize {
		r.needsUpdateFingerprint = true
	}
	return n, err
}

func (r *Reader) NameEquals(other *Reader) bool {
	return r.fileName == other.fileName
}

// Validate returns true if the reader still has a valid file handle, false otherwise.
func (r *Reader) Validate() bool {
	if r.file == nil {
		return false
	}
	refreshedFingerprint, err := fingerprint.NewFromFile(r.file, r.fingerprintSize, r.compression != "", r.set.Logger)
	if err != nil {
		return false
	}
	return refreshedFingerprint.StartsWith(r.Fingerprint)
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
	refreshedFingerprint, err := fingerprint.NewFromFile(r.file, r.fingerprintSize, r.compression != "", r.set.Logger)
	if err != nil {
		return
	}
	if r.Fingerprint.Len() > 0 && !refreshedFingerprint.StartsWith(r.Fingerprint) {
		return // fingerprint tampered, likely due to truncation
	}
	r.Fingerprint = refreshedFingerprint
}

func (r *Reader) getBufPtrFromPool() *[]byte {
	bufP := r.bufPool.Get()
	if bufP == nil {
		buf := make([]byte, 0, r.initialBufferSize)
		return &buf
	}
	return bufP.(*[]byte)
}
