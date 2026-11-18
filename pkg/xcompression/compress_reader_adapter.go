// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xcompression"

import (
	"errors"
	"io"
)

// readAdapter allows for compression libraries that
// do not adhere to [ResettableReader] to be adapter.
// There is a performance penalty using the adapter and
// should only be used if required.
type readAdapter[R io.Reader] struct {
	reader R
	reset  func(R, io.Reader) error
	close  func(R) error
}

var _ ResettableReader = (*readAdapter[io.Reader])(nil)

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

// newReadAdapter wraps a reader and adapts its reset and close behavior to [ResettableReader].
func newReadAdapter[R io.Reader](concrete R, opts ...func(*readAdapter[R])) *readAdapter[R] {
	ra := &readAdapter[R]{
		reader: concrete,
		reset: func(_ R, _ io.Reader) error {
			return errors.New("reset not specified")
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
