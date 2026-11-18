// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xutilcompress

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadAdapterDelegatesOperations(t *testing.T) {
	t.Parallel()

	resetErr := errors.New("reset failed")
	closeErr := errors.New("close failed")
	reader := bytes.NewBufferString("adapter payload")
	var gotSource io.Reader
	var gotClosed bool
	adapter := newReadAdapter(reader,
		withReadAdapterReset(func(gotReader *bytes.Buffer, src io.Reader) error {
			assert.Same(t, reader, gotReader)
			gotSource = src
			return resetErr
		}),
		withReadAdapterClose(func(gotReader *bytes.Buffer) error {
			assert.Same(t, reader, gotReader)
			gotClosed = true
			return closeErr
		}),
	)

	got := make([]byte, len("adapter payload"))
	n, err := adapter.Read(got)
	require.NoError(t, err)
	assert.Equal(t, len(got), n)
	assert.Equal(t, "adapter payload", string(got))

	source := bytes.NewReader([]byte("next input"))
	assert.ErrorIs(t, adapter.Reset(source), resetErr)
	assert.Same(t, source, gotSource)

	assert.ErrorIs(t, adapter.Close(), closeErr)
	assert.True(t, gotClosed)
}

func TestReadAdapterDefaultResetAndClose(t *testing.T) {
	t.Parallel()

	reader := bytes.NewBufferString("adapter payload")
	adapter := newReadAdapter(reader)
	assert.NoError(t, adapter.Reset(bytes.NewReader(nil)))
	assert.NoError(t, adapter.Close())

	got, err := io.ReadAll(adapter)
	require.NoError(t, err)
	assert.Equal(t, "adapter payload", string(got))
}
