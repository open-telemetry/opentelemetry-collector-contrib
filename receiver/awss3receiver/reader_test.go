// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
)

type readFunc func([]byte) (int, error)

func (f readFunc) Read(p []byte) (int, error) { return f(p) }

func TestReadCancellation(t *testing.T) {
	tests := []struct {
		name string
		read func(context.Context, io.Reader) error
	}{
		{
			name: "buffer",
			read: func(ctx context.Context, r io.Reader) error {
				_, err := readAll(ctx, r, 0)
				return err
			},
		},
		{
			name: "discard",
			read: func(ctx context.Context, r io.Reader) error {
				return copyReader(ctx, io.Discard, r)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var reads int
			reader := readFunc(func(p []byte) (int, error) {
				reads++
				cancel()
				return copy(p, "data"), nil
			})
			require.ErrorIs(t, tt.read(ctx, reader), context.Canceled)
			require.Equal(t, 1, reads)
		})
	}
}

func TestReadAllLimit(t *testing.T) {
	failure := errors.New("download failed")
	tests := []struct {
		name    string
		reader  io.Reader
		limit   int64
		want    string
		wantErr error
	}{
		{
			name:   "below_limit",
			reader: strings.NewReader("abc"),
			limit:  4,
			want:   "abc",
		},
		{
			name:   "exact_limit",
			reader: strings.NewReader("abcd"),
			limit:  4,
			want:   "abcd",
		},
		{
			name:   "exact_limit_with_eof",
			reader: iotest.DataErrReader(strings.NewReader("abcd")),
			limit:  4,
			want:   "abcd",
		},
		{
			name:    "over_limit",
			reader:  strings.NewReader("abcde"),
			limit:   4,
			wantErr: errUnmarshalSizeLimit,
		},
		{
			name:   "default_limit",
			reader: strings.NewReader("abc"),
			want:   "abc",
		},
		{
			name:    "read_error_at_limit",
			reader:  io.MultiReader(strings.NewReader("abcd"), iotest.ErrReader(failure)),
			limit:   4,
			want:    "abcd",
			wantErr: failure,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := readAll(t.Context(), tt.reader, tt.limit)
			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.want, string(data))
		})
	}
}

func TestReadAllStopsAtLimit(t *testing.T) {
	const limit = 1024
	var bytesRead int
	input := readFunc(func(p []byte) (int, error) {
		bytesRead += len(p)
		return len(p), nil
	})
	data, err := readAll(t.Context(), input, limit)
	require.ErrorIs(t, err, errUnmarshalSizeLimit)
	require.Nil(t, data)
	require.Equal(t, limit+1, bytesRead)
}
