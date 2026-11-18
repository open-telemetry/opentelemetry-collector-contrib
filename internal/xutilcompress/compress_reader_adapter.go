// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xutilcompress // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/xutilcompress"

import "io"

type readAdapter[R io.Reader] struct {
	reader R
	reset  func(R, io.Reader) error
	close  func(R) error
}

var (
	_ resettableReader = (*readAdapter[io.Reader])(nil)
)

// withReadAdapterClose sets the close behavior for the wrapped reader.
func withReadAdapterClose[R io.Reader](closer func(R) error) func(*readAdapter[R]) {
	return func(ra *readAdapter[R]) {
		ra.close = closer
	}
}

// withReadAdapterReset sets the reset behavior for the wrapped reader.
func withReadAdapterReset[R io.Reader](resetter func(R, io.Reader) error) func(*readAdapter[R]) {
	return func(ra *readAdapter[R]) {
		ra.reset = resetter
	}
}

// newReadAdapter wraps a reader and adapts its reset and close behavior to
// resettableReader. Unspecified reset and close operations are no-ops.
func newReadAdapter[R io.Reader](concrete R, opts ...func(*readAdapter[R])) *readAdapter[R] {
	ra := &readAdapter[R]{
		reader: concrete,
		reset: func(r1 R, r2 io.Reader) error {
			return nil
		},
		close: func(_ R) error {
			return nil
		},
	}
	for _, opt := range opts {
		opt(ra)
	}
	return ra
}

// Read delegates to the wrapped reader.
func (ra *readAdapter[R]) Read(b []byte) (int, error) {
	return ra.reader.Read(b)
}

// Reset delegates to the configured reset function.
func (ra *readAdapter[R]) Reset(src io.Reader) error {
	return ra.reset(ra.reader, src)
}

// Close delegates to the configured close function.
func (ra *readAdapter[R]) Close() error {
	return ra.close(ra.reader)
}
