// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xcompression

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReadAdapter(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		opts     []func(*readAdapter[*bytes.Buffer])
		closeErr string
		resetErr string
	}{
		{
			name:     "default",
			opts:     nil,
			closeErr: "",
			resetErr: "reset not specified",
		},
		{
			name: "reset method provided",
			opts: []func(*readAdapter[*bytes.Buffer]){
				withReadAdapterReset(func(_ *bytes.Buffer, _ io.Reader) error {
					return nil
				}),
			},
			closeErr: "",
			resetErr: "",
		},
		{
			name: "all options provided",
			opts: []func(*readAdapter[*bytes.Buffer]){
				withReadAdapterReset(func(_ *bytes.Buffer, _ io.Reader) error {
					return nil
				}),
				withReadAdapterClose(func(*bytes.Buffer) error {
					return errors.New("boom")
				}),
			},
			closeErr: "boom",
			resetErr: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			adapt := newReadAdapter(bytes.NewBuffer(nil), tc.opts...)

			if err := adapt.Close(); tc.closeErr != "" {
				assert.EqualError(t, err, tc.closeErr, "Must match the expected error")
			} else {
				assert.NoError(t, err, "Must not report an error")
			}

			if err := adapt.Reset(bytes.NewBuffer(nil)); tc.resetErr != "" {
				assert.EqualError(t, err, tc.resetErr, "Must match the expected error message")
			} else {
				assert.NoError(t, err, "Must not report an error when resetting")
			}
		})
	}
}

func TestReadAdapterRead(t *testing.T) {
	t.Parallel()

	orig := bytes.NewBufferString("OpenTelemetry!")

	adapter := newReadAdapter(
		orig,
		withReadAdapterReset(func(b *bytes.Buffer, r io.Reader) error {
			b.Reset()
			_, err := io.Copy(b, r)
			return err
		}),
	)

	actual, err := io.ReadAll(adapter)
	require.NoError(t, err, "Must not error while reading data")
	assert.Equal(t, "OpenTelemetry!", string(actual), "Must match the expected content")

	require.NoError(t, adapter.Reset(strings.NewReader("Welcome!")), "Must not error when resetting adapter")

	actual, err = io.ReadAll(adapter)
	require.NoError(t, err, "Must not error while reading data")
	assert.Equal(t, "Welcome!", string(actual), "Must match the expected content")
}
