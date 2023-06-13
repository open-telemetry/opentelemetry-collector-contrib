// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"bufio"
	"io"

	"go.uber.org/multierr"
)

// bufferedWriteCloser is intended to use more memory
// in order to optimize writing to disk to help improve performance.
type bufferedWriteCloser struct {
	wrapped  io.Closer
	buffered *bufio.Writer
}

var (
	_ io.WriteCloser = (*bufferedWriteCloser)(nil)
)

func newBufferedWriteCloser(f io.WriteCloser) WriteCloseFlusher {
	return &bufferedWriteCloser{
		wrapped:  f,
		buffered: bufio.NewWriter(f),
	}
}

func (bwc *bufferedWriteCloser) Write(p []byte) (n int, err error) {
	return bwc.buffered.Write(p)
}

func (bwc *bufferedWriteCloser) Close() error {
	return multierr.Combine(
		bwc.buffered.Flush(),
		bwc.wrapped.Close(),
	)
}

func (bwc *bufferedWriteCloser) getWrapped() io.Closer {
	return bwc.wrapped
}

func (bwc *bufferedWriteCloser) Flush() error {
	return bwc.buffered.Flush()
}
