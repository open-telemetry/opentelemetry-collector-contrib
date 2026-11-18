package xutilcompress

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nopCompressErrorWriter struct {
	err error
}

func (w nopCompressErrorWriter) Write([]byte) (int, error) {
	return 0, w.err
}

type nopCompressShortWriter struct{}

func (nopCompressShortWriter) Write(p []byte) (int, error) {
	return len(p) - 1, nil
}

func TestNopCompressCopiesInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		copy func(io.Writer, io.Reader, []byte) (int64, error)
	}{
		{
			name: "compress reader",
			copy: func(dst io.Writer, src io.Reader, _ []byte) (int64, error) {
				return (NopCompress{}).Compress(dst, src)
			},
		},
		{
			name: "compress bytes",
			copy: func(dst io.Writer, _ io.Reader, b []byte) (int64, error) {
				return (NopCompress{}).CompressBytes(dst, b)
			},
		},
		{
			name: "decompress reader",
			copy: func(dst io.Writer, src io.Reader, _ []byte) (int64, error) {
				return (NopCompress{}).Decompress(dst, src)
			},
		},
		{
			name: "decompress bytes",
			copy: func(dst io.Writer, _ io.Reader, b []byte) (int64, error) {
				return (NopCompress{}).DecompressBytes(dst, b)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, input := range [][]byte{nil, []byte("pass through without changing bytes")} {
				var dst bytes.Buffer
				n, err := test.copy(&dst, bytes.NewReader(input), input)
				require.NoError(t, err)
				assert.Equal(t, int64(len(input)), n)
				assert.Equal(t, input, dst.Bytes())
			}
		})
	}
}

func TestNopCompressPropagatesDestinationErrors(t *testing.T) {
	wantErr := errors.New("destination failed")
	input := []byte("payload")

	tests := []struct {
		name string
		copy func(io.Writer) (int64, error)
	}{
		{
			name: "compress reader",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).Compress(dst, bytes.NewReader(input))
			},
		},
		{
			name: "compress bytes",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).CompressBytes(dst, input)
			},
		},
		{
			name: "decompress reader",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).Decompress(dst, bytes.NewReader(input))
			},
		},
		{
			name: "decompress bytes",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).DecompressBytes(dst, input)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.copy(nopCompressErrorWriter{err: wantErr})
			assert.ErrorIs(t, err, wantErr)
		})
	}
}

func TestNopCompressReturnsShortWriteError(t *testing.T) {
	input := []byte("payload")
	tests := []struct {
		name string
		copy func(io.Writer) (int64, error)
	}{
		{
			name: "compress reader",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).Compress(dst, bytes.NewReader(input))
			},
		},
		{
			name: "compress bytes",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).CompressBytes(dst, input)
			},
		},
		{
			name: "decompress reader",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).Decompress(dst, bytes.NewReader(input))
			},
		},
		{
			name: "decompress bytes",
			copy: func(dst io.Writer) (int64, error) {
				return (NopCompress{}).DecompressBytes(dst, input)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			n, err := test.copy(nopCompressShortWriter{})
			assert.Equal(t, int64(len(input)-1), n)
			assert.ErrorIs(t, err, io.ErrShortWrite)
		})
	}
}

func TestNopCompressPropagatesSourceErrors(t *testing.T) {
	wantErr := errors.New("source failed")
	tests := []struct {
		name string
		copy func(io.Reader) (int64, error)
	}{
		{
			name: "compress reader",
			copy: func(src io.Reader) (int64, error) {
				return (NopCompress{}).Compress(io.Discard, src)
			},
		},
		{
			name: "decompress reader",
			copy: func(src io.Reader) (int64, error) {
				return (NopCompress{}).Decompress(io.Discard, src)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.copy(iotest.ErrReader(wantErr))
			assert.ErrorIs(t, err, wantErr)
		})
	}
}
