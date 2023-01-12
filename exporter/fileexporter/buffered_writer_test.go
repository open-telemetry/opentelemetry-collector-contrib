package fileexporter

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

const (
	msg = "it is a beautiful world"
)

type NopWriteCloser struct {
	w io.Writer
}

func (NopWriteCloser) Close() error                    { return nil }
func (wc *NopWriteCloser) Write(p []byte) (int, error) { return wc.w.Write(p) }

func TestBufferedWrites(t *testing.T) {
	t.Parallel()

	b := bytes.NewBuffer(nil)
	w := newBufferedWriterCloser(&NopWriteCloser{b})

	_, err := w.Write([]byte(msg))
	require.NoError(t, err, "Must not error when writing data")
	assert.NoError(t, w.Close(), "Must not error when closing writer")

	assert.Equal(t, msg, b.String(), "Must match the expected string")
}

var (
	benchmarkErr error
)

func BenchmarkWriter(b *testing.B) {
	tempfile := func(tb testing.TB) io.WriteCloser {
		f, err := ioutil.TempFile(tb.TempDir(), tb.Name())
		require.NoError(tb, err, "Must not error when creating benchmark temp file")
		tb.Cleanup(func() {
			assert.NoError(tb, os.RemoveAll(path.Dir(f.Name())), "Must clean up files after being writen")
		})
		return f
	}

	for name, w := range map[string]io.WriteCloser{
		"discard":          &NopWriteCloser{io.Discard},
		"buffered-discard": newBufferedWriterCloser(&NopWriteCloser{io.Discard}),
		"raw-file":         tempfile(b),
		"buffered-file":    newBufferedWriterCloser(tempfile(b)),
	} {
		w := w
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()

			var err error
			for i := 0; i < b.N; i++ {
				_, err = w.Write([]byte(msg))
			}
			benchmarkErr = multierr.Combine(err, w.Close())
		})
	}
}
