// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"sync"
)

var (
	errOverCapacity = errors.New("over capacity")
)

type buffer interface {
	io.Writer
	io.Reader
	io.Closer
	Reset()
	Len() int
	Empty() bool
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

func (c *cancellableBytesWriter) Empty() bool {
	return c.innerWriter.Len() == 0
}

type cancellableGzipWriter struct {
	innerBuffer *bytes.Buffer
	innerWriter *gzip.Writer
	maxCapacity uint
	rawLen      int
}

func (c *cancellableGzipWriter) Write(b []byte) (int, error) {
	if c.maxCapacity == 0 {
		c.rawLen += len(b)
		return c.innerWriter.Write(b)
	}

	// if we see that at a 50% compression rate, we'd be over max capacity, start flushing.
	if c.rawLen > 0 && (c.rawLen+len(b))/2 > int(c.maxCapacity) {
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

	c.rawLen += len(b)
	return c.innerWriter.Write(b)
}

func (c *cancellableGzipWriter) Read(p []byte) (int, error) {
	return c.innerBuffer.Read(p)
}

func (c *cancellableGzipWriter) Reset() {
	c.innerBuffer.Reset()
	c.innerWriter.Reset(c.innerBuffer)
	c.rawLen = 0
}

func (c *cancellableGzipWriter) Close() error {
	return c.innerWriter.Close()
}

func (c *cancellableGzipWriter) Len() int {
	return c.innerBuffer.Len()
}

func (c *cancellableGzipWriter) Empty() bool {
	return c.rawLen == 0
}

// bufferPool is a pool of buffer objects.
type bufferPool struct {
	pool *sync.Pool
}

func (p bufferPool) get() buffer {
	return p.pool.Get().(buffer)
}

func (p bufferPool) put(bf buffer) {
	p.pool.Put(bf)
}

func newBufferPool(bufCap uint, compressionEnabled bool) bufferPool {
	return bufferPool{
		&sync.Pool{
			New: func() any {
				innerBuffer := &bytes.Buffer{}
				if compressionEnabled {
					return &cancellableGzipWriter{
						innerBuffer: innerBuffer,
						innerWriter: gzip.NewWriter(innerBuffer),
						maxCapacity: bufCap,
					}
				}
				return &cancellableBytesWriter{
					innerWriter: innerBuffer,
					maxCapacity: bufCap,
				}
			},
		},
	}
}
