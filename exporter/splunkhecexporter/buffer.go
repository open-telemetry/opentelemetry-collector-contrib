package splunkhecexporter

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"sync"

	"go.opentelemetry.io/collector/pdata/plog"
)

var (
	errOverCapacity = errors.New("over capacity")
)

// Minimum number of bytes to compress. 1500 is the MTU of an ethernet frame.
const minCompressionLen = 1500

// Composite index of a record.
type index struct {
	// Index in orig list (i.e. root parent index).
	resource int
	// Index in ScopeLogs/ScopeMetrics list (i.e. immediate parent index).
	library int
	// Index in Logs list (i.e. the log record index).
	record int
}

// bufferState encapsulates intermediate buffer state when pushing data
type bufferState struct {
	compressionAvailable bool
	compressionEnabled   bool
	bufferMaxLen         uint
	writer               io.Writer
	buf                  *bytes.Buffer
	bufFront             *index
	resource             int
	library              int
	gzipWriterPool       *sync.Pool
	mu                   sync.Mutex
}

func (b *bufferState) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
	b.compressionEnabled = false
	b.writer = &cancellableBytesWriter{innerWriter: b.buf, maxCapacity: b.bufferMaxLen}
}

func (b *bufferState) Read(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Read(p)
}

func (b *bufferState) copyBuffer() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	buffer := make([]byte, b.buf.Len())
	copy(buffer, b.buf.Bytes())
	return buffer
}

func (b *bufferState) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.writer.(*cancellableGzipWriter); ok {
		return b.writer.(*cancellableGzipWriter).close()
	}
	return nil
}

// accept returns true if data is accepted by the buffer
func (b *bufferState) accept(data []byte) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, err := b.writer.Write(data)
	overCapacity := errors.Is(err, errOverCapacity)
	bufLen := b.buf.Len()
	if overCapacity {
		bufLen += len(data)
	}
	if b.compressionAvailable && !b.compressionEnabled && bufLen > minCompressionLen {
		// switch over to a zip buffer.
		tmpBuf := bytes.NewBuffer(make([]byte, 0, b.bufferMaxLen+bufCapPadding))
		writer := b.gzipWriterPool.Get().(*gzip.Writer)
		writer.Reset(tmpBuf)
		zipWriter := &cancellableGzipWriter{
			innerBuffer: tmpBuf,
			innerWriter: writer,
			// 8 bytes required for the zip footer.
			maxCapacity:    b.bufferMaxLen - 8,
			gzipWriterPool: b.gzipWriterPool,
		}

		// the new data is so big, even with a zip writer, we are over the max limit.
		// abandon and return false, so we can send what is already in our buffer.
		if _, err2 := zipWriter.Write(b.buf.Bytes()); err2 != nil {
			return false, err2
		}
		b.writer = zipWriter
		b.buf = tmpBuf
		b.compressionEnabled = true
		// if the byte writer was over capacity, try to write the new entry in the zip writer:
		if overCapacity {
			if _, err2 := zipWriter.Write(data); err2 != nil {
				return false, err2
			}

		}
		return true, nil
	}
	if overCapacity {
		return false, nil
	}
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
	innerBuffer    *bytes.Buffer
	innerWriter    *gzip.Writer
	maxCapacity    uint
	gzipWriterPool *sync.Pool
	len            int
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
		writerCopy := c.gzipWriterPool.Get().(*gzip.Writer)
		defer c.gzipWriterPool.Put(writerCopy)
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
	err := c.innerWriter.Close()
	c.gzipWriterPool.Put(c.innerWriter)
	return err
}

// A guesstimated value > length of bytes of a single event.
// Added to buffer capacity so that buffer is likely to grow by reslicing when buf.Len() > bufCap.
const bufCapPadding = uint(4096)
const libraryHeaderName = "X-Splunk-Instrumentation-Library"
const profilingLibraryName = "otel.profiling"

var profilingHeaders = map[string]string{
	libraryHeaderName: profilingLibraryName,
}

func isProfilingData(sl plog.ScopeLogs) bool {
	return sl.Scope().Name() == profilingLibraryName
}

func makeBlankBufferState(bufCap uint, compressionAvailable bool, pool *sync.Pool) *bufferState {
	// Buffer of JSON encoded Splunk events, last record is expected to overflow bufCap, hence the padding
	buf := bytes.NewBuffer(make([]byte, 0, bufCap+bufCapPadding))

	return &bufferState{
		compressionAvailable: compressionAvailable,
		compressionEnabled:   false,
		writer:               &cancellableBytesWriter{innerWriter: buf, maxCapacity: bufCap},
		buf:                  buf,
		bufferMaxLen:         bufCap,
		bufFront:             nil, // Index of the log record of the first unsent event in buffer.
		resource:             0,   // Index of currently processed Resource
		library:              0,   // Index of currently processed Library
		gzipWriterPool:       pool,
		mu:                   sync.Mutex{},
	}
}
