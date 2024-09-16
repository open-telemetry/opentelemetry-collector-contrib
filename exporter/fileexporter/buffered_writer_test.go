// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	msg = "it is a beautiful world"

	SizeByte     = 1
	SizeKiloByte = 1 << (10 * iota)
	SizeMegaByte
)

type NopWriteCloser struct {
	w io.Writer
}

func (NopWriteCloser) Close() error                    { return nil }
func (wc *NopWriteCloser) Write(p []byte) (int, error) { return wc.w.Write(p) }

func TestBufferedWrites(t *testing.T) {
	t.Parallel()

	b := bytes.NewBuffer(nil)
	w := newBufferedWriteCloser(&NopWriteCloser{b})

	_, err := w.Write([]byte(msg))
	require.NoError(t, err, "Must not error when writing data")
	assert.NoError(t, w.Close(), "Must not error when closing writer")

	assert.Equal(t, msg, b.String(), "Must match the expected string")
}

var (
	errBenchmark error
)

func BenchmarkWriter(b *testing.B) {
	tempfile := func(tb testing.TB) io.WriteCloser {
		f, err := os.CreateTemp(tb.TempDir(), tb.Name())
		require.NoError(tb, err, "Must not error when creating benchmark temp file")
		tb.Cleanup(func() {
			assert.NoError(tb, os.RemoveAll(path.Dir(f.Name())), "Must clean up files after being written")
		})
		return f
	}

	for _, payloadSize := range []int{
		10 * SizeKiloByte,
		100 * SizeKiloByte,
		SizeMegaByte,
		10 * SizeMegaByte,
	} {
		payload := make([]byte, payloadSize)
		for i := 0; i < payloadSize; i++ {
			payload[i] = 'a'
		}
		for name, w := range map[string]io.WriteCloser{
			"discard":          &NopWriteCloser{io.Discard},
			"buffered-discard": newBufferedWriteCloser(&NopWriteCloser{io.Discard}),
			"raw-file":         tempfile(b),
			"buffered-file":    newBufferedWriteCloser(tempfile(b)),
		} {
			w := w
			b.Run(fmt.Sprintf("%s_%d_bytes", name, payloadSize), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				var err error
				for i := 0; i < b.N; i++ {
					_, err = w.Write(payload)
				}
				errBenchmark = errors.Join(err, w.Close())
			})
		}
	}

}
