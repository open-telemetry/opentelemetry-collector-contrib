// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package compress_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/compress"
)

func TestCompressorFormats(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		format string
	}{
		{format: "none"},
		{format: "noop"},
		{format: "gzip"},
		{format: "zlib"},
		{format: "flate"},
	}

	data := createRandomString(1024)

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("format_%s", tc.format), func(t *testing.T) {
			c, err := compress.NewCompressor(tc.format)
			require.NoError(t, err, "Must have a valid compression format")
			require.NotNil(t, c, "Must have a valid compressor")

			out, err := c([]byte(data))
			assert.NoError(t, err, "Must not error when processing data")
			assert.NotNil(t, out, "Must have a valid record")

			// now data gets decompressed and the original string gets compared with the decompressed one
			var dc []byte
			var err2 error

			switch tc.format {
			case "gzip":
				dc, err2 = decompressGzip(out)
			case "zlib":
				dc, err2 = decompressZlib(out)
			case "flate":
				dc, err2 = decompressFlate(out)
			case "noop", "none":
				dc = out
			default:
				dc = out
			}

			assert.NoError(t, err2)
			assert.Equal(t, data, string(dc))
		})
	}
	_, err := compress.NewCompressor("invalid-format")
	assert.Error(t, err, "Must error when an invalid compression format is given")
}

func createRandomString(length int) string {
	// some characters for the random generation
	const letterBytes = " ,.;:*-+/[]{}<>abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	b := make([]byte, length)
	for i := range b {
		b[i] = letterBytes[rand.IntN(len(letterBytes))]
	}

	return string(b)
}

func BenchmarkNoopCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "none", 1000)
}

func BenchmarkNoopCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "noop", 131072)
}

func BenchmarkZlibCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "zlib", 1000)
}

func BenchmarkZlibCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "zlib", 131072)
}

func BenchmarkFlateCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "flate", 1000)
}

func BenchmarkFlateCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "flate", 131072)
}

func BenchmarkGzipCompressor_1000Bytes(b *testing.B) {
	benchmarkCompressor(b, "gzip", 1000)
}

func BenchmarkGzipCompressor_1Mb(b *testing.B) {
	benchmarkCompressor(b, "gzip", 131072)
}

func benchmarkCompressor(b *testing.B, format string, length int) {
	b.Helper()

	compressor, err := compress.NewCompressor(format)
	require.NoError(b, err, "Must not error when given a valid format")
	require.NotNil(b, compressor, "Must have a valid compressor")

	data := make([]byte, length)
	for i := range length {
		data[i] = byte(rand.Int32())
	}
	b.ReportAllocs()

	for b.Loop() {
		out, err := compressor(data)
		assert.NoError(b, err, "Must not error when processing data")
		assert.NotNil(b, out, "Must have a valid byte array after")
	}
}

// an issue encountered in the past was a crash due race condition in the compressor, so the
// current implementation creates a new context on each compression request
// this is a test to check no exceptions are raised for executing concurrent compressions
func TestCompressorConcurrent(t *testing.T) {
	timeout := time.After(15 * time.Second)
	done := make(chan bool)
	go func() {
		// do your testing
		concurrentCompressFunc(t)
		done <- true
	}()

	select {
	case <-timeout:
		t.Fatal("Test didn't finish in time")
	case <-done:
	}
}

func concurrentCompressFunc(t *testing.T) {
	// this value should be way higher to make this test more valuable, but the make of this project uses
	// max 4 workers, so we had to set this value here
	numWorkers := 4

	var wg sync.WaitGroup
	wg.Add(numWorkers)

	errCh := make(chan error, numWorkers)
	var errMutex sync.Mutex

	// any single format would do it here, since each exporter can be set to use only one at a time
	// and the concurrent issue that was present in the past was independent of the format
	compressFunc, err := compress.NewCompressor("gzip")
	if err != nil {
		errCh <- err
		return
	}

	// it is important for the data length to be on the higher side of a record
	// since it is where the chances of having race conditions are bigger
	dataLength := 131072

	for range numWorkers {
		go func() {
			defer wg.Done()

			data := make([]byte, dataLength)
			for i := range dataLength {
				data[i] = byte(rand.Int32())
			}

			result, localErr := compressFunc(data)
			if localErr != nil {
				errMutex.Lock()
				errCh <- localErr
				errMutex.Unlock()
				return
			}

			_ = result
		}()
	}

	wg.Wait()

	close(errCh)

	for err := range errCh {
		t.Errorf("Error encountered on concurrent compression: %v", err)
	}
}

func decompressGzip(input []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}

	defer r.Close()

	decompressedData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return decompressedData, nil
}

func decompressZlib(input []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}

	defer r.Close()

	decompressedData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return decompressedData, nil
}

func decompressFlate(input []byte) ([]byte, error) {
	r := flate.NewReader(bytes.NewReader(input))
	defer r.Close()

	decompressedData, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return decompressedData, nil
}
