// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import "io"

type passthroughWriter struct {
	io.Writer
}

var _ ResettableWriter = (*passthroughWriter)(nil)

func (pw *passthroughWriter) Reset(w io.Writer) {
	pw.Writer = w
}

func (pw *passthroughWriter) Close() error {
	if closer, ok := pw.Writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

type passthroughReader struct {
	io.Reader
}

var _ ResettableReader = (*passthroughReader)(nil)

func (pr *passthroughReader) Reset(r io.Reader) error {
	pr.Reader = r
	return nil
}

func (pr *passthroughReader) Close() error {
	if closer, ok := pr.Reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
