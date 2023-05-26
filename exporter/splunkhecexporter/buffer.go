// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

var (
	errOverCapacity = errors.New("over capacity")
)

// Minimum number of bytes to compress. 1500 is the MTU of an ethernet frame.
const minCompressionLen = 1500

// bufferState encapsulates intermediate buffer state when pushing data
type bufferState struct {
	compressionAvailable bool
	bufferMaxLen         uint
	maxEventLength       uint
	writer               io.Writer
	buf                  *bytes.Buffer
	jsonStream           *jsoniter.Stream
	rawLength            int
}

func (b *bufferState) compressionEnabled() bool {
	_, ok := b.writer.(*cancellableGzipWriter)
	return ok
}

func (b *bufferState) containsData() bool {
	return b.rawLength > 0
}

func (b *bufferState) reset() {
	b.buf.Reset()
	if _, ok := b.writer.(*cancellableBytesWriter); !ok {
		b.writer = &cancellableBytesWriter{innerWriter: b.buf, maxCapacity: b.bufferMaxLen}
	}
	b.rawLength = 0
}

func (b *bufferState) Read(p []byte) (n int, err error) {
	return b.buf.Read(p)
}

func (b *bufferState) Close() error {
	if _, ok := b.writer.(*cancellableGzipWriter); ok {
		return b.writer.(*cancellableGzipWriter).close()
	}
	return nil
}

// accept returns true if data is accepted by the buffer
func (b *bufferState) accept(data []byte) (bool, error) {
	if len(data) > int(b.maxEventLength) {
		return false, nil
	}
	_, err := b.writer.Write(data)
	overCapacity := errors.Is(err, errOverCapacity)
	bufLen := b.buf.Len()
	if overCapacity {
		bufLen += len(data)
	}
	if b.compressionAvailable && !b.compressionEnabled() && bufLen > minCompressionLen {
		// switch over to a zip buffer.
		tmpBuf := bytes.NewBuffer(make([]byte, 0, b.bufferMaxLen+bufCapPadding))
		writer := gzip.NewWriter(tmpBuf)
		writer.Reset(tmpBuf)
		zipWriter := &cancellableGzipWriter{
			innerBuffer: tmpBuf,
			innerWriter: writer,
			// 8 bytes required for the zip footer.
			maxCapacity: b.bufferMaxLen - 8,
		}

		if b.bufferMaxLen == 0 {
			zipWriter.maxCapacity = 0
		}

		// we write the bytes buffer into the zip buffer. Any error from this is I/O, and should stop the process.
		if _, err2 := zipWriter.Write(b.buf.Bytes()); err2 != nil {
			return false, err2
		}
		b.writer = zipWriter
		b.buf = tmpBuf
		// if the byte writer was over capacity, try to write the new entry in the zip writer:
		if overCapacity {
			if _, err2 := zipWriter.Write(data); err2 != nil {
				overCapacity2 := errors.Is(err2, errOverCapacity)
				if overCapacity2 {
					return false, nil
				}
				return false, err2
			}

		}
		b.rawLength += len(data)
		return true, nil
	}
	if overCapacity {
		return false, nil
	}
	b.rawLength += len(data)
	return true, err
}

type cancellableBytesWriter struct {
	innerWriter *bytes.Buffer
	maxCapacity uint
}

func (c *cancellableBytesWriter) Write(b []byte) (int, error) {
	if c.maxCapacity == 0 {
		return c.innerWriter.Write(b)
	}
	if c.innerWriter.Len()+len(b) > int(c.maxCapacity) {
		return 0, errOverCapacity
	}
	return c.innerWriter.Write(b)
}

type cancellableGzipWriter struct {
	innerBuffer *bytes.Buffer
	innerWriter *gzip.Writer
	maxCapacity uint
	len         int
}

func (c *cancellableGzipWriter) Write(b []byte) (int, error) {
	if c.maxCapacity == 0 {
		return c.innerWriter.Write(b)
	}
	c.len += len(b)
	// if we see that at a 50% compression rate, we'd be over max capacity, start flushing.
	if (c.len / 2) > int(c.maxCapacity) {
		// we flush so the length of the underlying buffer is accurate.
		if err := c.innerWriter.Flush(); err != nil {
			return 0, err
		}
	}
	// we find that the new content uncompressed, added to our buffer, would overflow our max capacity.
	if c.innerBuffer.Len()+len(b) > int(c.maxCapacity) {
		// so we create a copy of our content and add this new data, compressed, to check that it fits.
		copyBuf := bytes.NewBuffer(make([]byte, 0, c.maxCapacity+bufCapPadding))
		copyBuf.Write(c.innerBuffer.Bytes())
		writerCopy := gzip.NewWriter(copyBuf)
		writerCopy.Reset(copyBuf)
		if _, err := writerCopy.Write(b); err != nil {
			return 0, err
		}
		if err := writerCopy.Flush(); err != nil {
			return 0, err
		}
		// we find that even compressed, the data overflows.
		if copyBuf.Len() > int(c.maxCapacity) {
			return 0, errOverCapacity
		}
	}
	return c.innerWriter.Write(b)
}

func (c *cancellableGzipWriter) close() error {
	return c.innerWriter.Close()
}

// bufferStatePool is a pool of bufferState objects.
type bufferStatePool struct {
	pool *sync.Pool
}

// get returns a bufferState from the pool.
func (p bufferStatePool) get() *bufferState {
	return p.pool.Get().(*bufferState)
}

// put returns a bufferState to the pool.
func (p bufferStatePool) put(bf *bufferState) {
	p.pool.Put(bf)
}

const initBufferCap = 512

func newBufferStatePool(bufCap uint, compressionAvailable bool, maxEventLength uint) bufferStatePool {
	return bufferStatePool{
		&sync.Pool{
			New: func() interface{} {
				buf := bytes.NewBuffer(make([]byte, 0, initBufferCap))
				return &bufferState{
					compressionAvailable: compressionAvailable,
					writer:               &cancellableBytesWriter{innerWriter: buf, maxCapacity: bufCap},
					buf:                  buf,
					jsonStream:           jsoniter.NewStream(jsoniter.ConfigDefault, nil, initBufferCap),
					bufferMaxLen:         bufCap,
					maxEventLength:       maxEventLength,
				}
			},
		},
	}
}
