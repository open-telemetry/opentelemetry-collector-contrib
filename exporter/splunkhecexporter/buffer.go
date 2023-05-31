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

// bufferState encapsulates intermediate buffer state when pushing data
type bufferState struct {
	maxEventLength uint
	buf            buffer
	jsonStream     *jsoniter.Stream
	rawLength      int
}

type buffer interface {
	io.Writer
	io.Reader
	io.Closer
	Reset()
	Len() int
}

func (b *bufferState) compressionEnabled() bool {
	_, ok := b.buf.(*cancellableGzipWriter)
	return ok
}

func (b *bufferState) containsData() bool {
	return b.rawLength > 0
}

func (b *bufferState) reset() {
	b.buf.Reset()
	b.rawLength = 0
}

func (b *bufferState) Read(p []byte) (n int, err error) {
	return b.buf.Read(p)
}

func (b *bufferState) Close() error {
	return b.buf.Close()
}

// accept returns true if data is accepted by the buffer
func (b *bufferState) accept(data []byte) (bool, error) {
	if len(data)+b.rawLength > int(b.maxEventLength) {
		return false, nil
	}
	_, err := b.buf.Write(data)
	if err == nil {
		b.rawLength += len(data)
		return true, nil
	}
	if errors.Is(err, errOverCapacity) {
		return false, nil
	}
	return false, err
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

func (c *cancellableBytesWriter) Read(p []byte) (int, error) {
	return c.innerWriter.Read(p)
}

func (c *cancellableBytesWriter) Reset() {
	c.innerWriter.Reset()
}

func (c *cancellableBytesWriter) Close() error {
	return nil
}

func (c *cancellableBytesWriter) Len() int {
	return c.innerWriter.Len()
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

func (c *cancellableGzipWriter) Read(p []byte) (int, error) {
	return c.innerBuffer.Read(p)
}

func (c *cancellableGzipWriter) Reset() {
	c.innerBuffer.Reset()
	c.innerWriter.Reset(c.innerBuffer)
	c.len = 0
}

func (c *cancellableGzipWriter) Close() error {
	return c.innerWriter.Close()
}

func (c *cancellableGzipWriter) Len() int {
	return c.innerBuffer.Len()
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

func newBufferStatePool(bufCap uint, compressionEnabled bool, maxEventLength uint) bufferStatePool {
	return bufferStatePool{
		&sync.Pool{
			New: func() interface{} {
				innerBuffer := bytes.NewBuffer(make([]byte, 0, initBufferCap))
				var buf buffer
				if compressionEnabled {
					buf = &cancellableGzipWriter{
						innerBuffer: innerBuffer,
						innerWriter: gzip.NewWriter(buf),
						maxCapacity: bufCap,
					}
				} else {
					buf = &cancellableBytesWriter{
						innerWriter: innerBuffer,
						maxCapacity: bufCap,
					}
				}
				return &bufferState{
					buf:            buf,
					jsonStream:     jsoniter.NewStream(jsoniter.ConfigDefault, nil, initBufferCap),
					maxEventLength: maxEventLength,
				}
			},
		},
	}
}
