package fileexporter

import (
	"bufio"
	"io"

	"go.uber.org/multierr"
)

// bufferedWriteCloser is intended to use more memory
// in order to optimise writing to disk to help improve performance.
type bufferedWriteCloser struct {
	wraped   io.Closer
	buffered *bufio.Writer
}

var (
	_ io.WriteCloser = (*bufferedWriteCloser)(nil)
)

func newBufferedWriterCloser(f io.WriteCloser) io.WriteCloser {
	return &bufferedWriteCloser{
		wraped:   f,
		buffered: bufio.NewWriter(f),
	}
}

func (bwc *bufferedWriteCloser) Write(p []byte) (n int, err error) {
	return bwc.buffered.Write(p)
}

func (bwc *bufferedWriteCloser) Close() error {
	return multierr.Combine(
		bwc.buffered.Flush(),
		bwc.wraped.Close(),
	)
}
