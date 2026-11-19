// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPassthroughReader(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("stream failed")
	for _, tc := range []struct {
		name      string
		input     string
		readErr   error
		withClose bool
		closeErr  error
	}{
		{name: "empty"},
		{name: "text", input: "payload"},
		{name: "binary", input: "\x00\xff\x80\x01"},
		{name: "read error", readErr: wantErr},
		{name: "closer", input: "payload", withClose: true},
		{name: "close error", input: "payload", withClose: true, closeErr: wantErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var source io.Reader = strings.NewReader(tc.input)
			if tc.readErr != nil {
				source = iotest.ErrReader(tc.readErr)
			}
			closer := &passthroughTestStream{Reader: source, closeErr: tc.closeErr}
			if tc.withClose {
				source = closer
			}

			r := passthroughReader{Reader: strings.NewReader("previous source")}
			require.NoError(t, r.Reset(source))
			actual, err := io.ReadAll(&r)
			assert.ErrorIs(t, err, tc.readErr)
			assert.Equal(t, tc.input, string(actual))
			assert.ErrorIs(t, r.Close(), tc.closeErr)
			assert.Equal(t, tc.withClose, closer.closed)
		})
	}
}

func TestPassthroughWriter(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("stream failed")
	for _, tc := range []struct {
		name      string
		input     string
		written   int
		writeErr  error
		withClose bool
		closeErr  error
	}{
		{name: "empty"},
		{name: "text", input: "payload", written: 7},
		{name: "binary", input: "\x00\xff\x80\x01", written: 4},
		{name: "write error", input: "payload", writeErr: wantErr},
		{name: "closer", input: "payload", written: 7, withClose: true},
		{name: "close error", input: "payload", written: 7, withClose: true, closeErr: wantErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			var destination io.Writer = &output
			if tc.writeErr != nil {
				destination = nopCompressionErrorWriter{err: tc.writeErr}
			}
			closer := &passthroughTestStream{Writer: destination, closeErr: tc.closeErr}
			if tc.withClose {
				destination = closer
			}

			var previous bytes.Buffer
			w := passthroughWriter{Writer: &previous}
			w.Reset(destination)
			n, err := w.Write([]byte(tc.input))
			assert.ErrorIs(t, err, tc.writeErr)
			assert.Equal(t, tc.written, n)
			assert.Equal(t, tc.input[:tc.written], output.String())
			assert.Empty(t, previous.Bytes())
			assert.ErrorIs(t, w.Close(), tc.closeErr)
			assert.Equal(t, tc.withClose, closer.closed)
		})
	}
}

type passthroughTestStream struct {
	io.Reader
	io.Writer
	closeErr error
	closed   bool
}

func (s *passthroughTestStream) Close() error {
	s.closed = true
	return s.closeErr
}
