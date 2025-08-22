package pool

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBufferPool(t *testing.T) {
	t.Helper()
	pool := NewBufferPool()

	assert.NotNil(t, pool)
	assert.NotNil(t, pool.pool)
}

func TestBufferPool_NewPooledBuffer(t *testing.T) {
	pool := NewBufferPool()

	t.Run("creates new pooled buffer", func(t *testing.T) {
		pooledBuf := pool.NewPooledBuffer()

		assert.NotNil(t, pooledBuf.Buffer)
		assert.NotNil(t, pooledBuf.pool)
		assert.Equal(t, 0, pooledBuf.Buffer.Len())
	})

	t.Run("multiple buffers from same pool", func(t *testing.T) {
		buf1 := pool.NewPooledBuffer()
		buf2 := pool.NewPooledBuffer()

		// Both should have valid buffers
		assert.NotNil(t, buf1.Buffer)
		assert.NotNil(t, buf2.Buffer)

		// They should reference the same pool
		assert.Equal(t, buf1.pool, buf2.pool)
	})
}

func TestPooledBuffer_Recycle(t *testing.T) {
	pool := NewBufferPool()

	testCases := []struct {
		name           string
		setupBuffer    func() (PooledBuffer, *bytes.Buffer)
		validateResult func(PooledBuffer, *bytes.Buffer)
	}{
		{
			name: "recycle clears buffer content",
			setupBuffer: func() (PooledBuffer, *bytes.Buffer) {
				pooledBuf := pool.NewPooledBuffer()
				testData := "test data"
				pooledBuf.Buffer.WriteString(testData)
				assert.Equal(t, len(testData), pooledBuf.Buffer.Len())
				return pooledBuf, nil
			},
			validateResult: func(pooledBuf PooledBuffer, originalBuffer *bytes.Buffer) {
				// Buffer should be reset after recycling
				assert.Equal(t, 0, pooledBuf.Buffer.Len())
			},
		},
		{
			name: "recycled buffer is reused",
			setupBuffer: func() (PooledBuffer, *bytes.Buffer) {
				pooledBuf1 := pool.NewPooledBuffer()
				originalBuffer := pooledBuf1.Buffer
				pooledBuf1.Buffer.WriteString("some data")
				return pooledBuf1, originalBuffer
			},
			validateResult: func(pooledBuf PooledBuffer, originalBuffer *bytes.Buffer) {
				// Get another buffer - it should be the same underlying buffer
				newPooledBuf := pool.NewPooledBuffer()
				assert.Same(t, originalBuffer, newPooledBuf.Buffer)
				assert.Equal(t, 0, newPooledBuf.Buffer.Len()) // Should be reset
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pooledBuf, originalBuffer := tc.setupBuffer()

			// Recycle the buffer
			pooledBuf.Recycle()

			// Validate the results
			tc.validateResult(pooledBuf, originalBuffer)
		})
	}
}

func TestPooledBuffer_WriteTo(t *testing.T) {
	pool := NewBufferPool()

	testCases := []struct {
		name           string
		setupBuffer    func() PooledBuffer
		setupWriter    func() io.Writer
		expectedBytes  int64
		expectedOutput string
		expectError    bool
		expectedError  error
	}{
		{
			name: "writes content and recycles buffer",
			setupBuffer: func() PooledBuffer {
				pooledBuf := pool.NewPooledBuffer()
				testData := "Hello, World!"
				pooledBuf.Buffer.WriteString(testData)
				return pooledBuf
			},
			setupWriter: func() io.Writer {
				return &bytes.Buffer{}
			},
			expectedBytes:  13, // len("Hello, World!")
			expectedOutput: "Hello, World!",
			expectError:    false,
		},
		{
			name: "handles empty buffer",
			setupBuffer: func() PooledBuffer {
				return pool.NewPooledBuffer()
			},
			setupWriter: func() io.Writer {
				return &bytes.Buffer{}
			},
			expectedBytes:  0,
			expectedOutput: "",
			expectError:    false,
		},
		{
			name: "handles writer error",
			setupBuffer: func() PooledBuffer {
				pooledBuf := pool.NewPooledBuffer()
				pooledBuf.Buffer.WriteString("test data")
				return pooledBuf
			},
			setupWriter: func() io.Writer {
				return &errorWriter{err: io.ErrShortWrite}
			},
			expectedBytes: 0,
			expectError:   true,
			expectedError: io.ErrShortWrite,
		},
		{
			name: "writes large content",
			setupBuffer: func() PooledBuffer {
				pooledBuf := pool.NewPooledBuffer()
				largeData := strings.Repeat("Hello, World! ", 1000)
				pooledBuf.Buffer.WriteString(largeData)
				return pooledBuf
			},
			setupWriter: func() io.Writer {
				return &bytes.Buffer{}
			},
			expectedBytes:  13000, // len("Hello, World! ") * 1000
			expectedOutput: strings.Repeat("Hello, World! ", 1000),
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pooledBuf := tc.setupBuffer()
			writer := tc.setupWriter()

			n, err := pooledBuf.WriteTo(writer)

			if tc.expectError {
				assert.Error(t, err)
				if tc.expectedError != nil {
					assert.Equal(t, tc.expectedError, err)
				}
			} else {
				require.NoError(t, err)

				// Check output content for successful writes
				if buf, ok := writer.(*bytes.Buffer); ok {
					assert.Equal(t, tc.expectedOutput, buf.String())
				}
			}

			assert.Equal(t, tc.expectedBytes, n)

			// Buffer should always be recycled (reset) after WriteTo, even on error
			assert.Equal(t, 0, pooledBuf.Buffer.Len())
		})
	}
}

func TestBufferPoolConcurrency(t *testing.T) {
	pool := NewBufferPool()

	t.Run("concurrent access to pool", func(t *testing.T) {
		const numGoroutines = 100
		const numIterations = 50

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numIterations; j++ {
					pooledBuf := pool.NewPooledBuffer()
					data := fmt.Sprintf("goroutine-%d-iter-%d", id, j)
					pooledBuf.Buffer.WriteString(data)

					var output bytes.Buffer
					n, err := pooledBuf.WriteTo(&output)

					assert.NoError(t, err)
					assert.Equal(t, int64(len(data)), n)
					assert.Equal(t, data, output.String())
				}
			}(i)
		}

		wg.Wait()
	})
}

// Helper type for testing error conditions
type errorWriter struct {
	err error
}

func (e *errorWriter) Write(_ []byte) (n int, err error) {
	return 0, e.err
}
