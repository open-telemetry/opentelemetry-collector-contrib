// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"

import (
	"bufio"
	"errors"
	"io"
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

func newBufferedWriteCloser(f io.WriteCloser) io.WriteCloser {
	return &bufferedWriteCloser{
		wrapped:  f,
		buffered: bufio.NewWriter(f),
	}
}

func (bwc *bufferedWriteCloser) Write(p []byte) (n int, err error) {
	return bwc.buffered.Write(p)
}

func (bwc *bufferedWriteCloser) Close() error {
	return errors.Join(
		bwc.buffered.Flush(),
		bwc.wrapped.Close(),
	)
}

func (bwc *bufferedWriteCloser) flush() error {
	return bwc.buffered.Flush()
}
