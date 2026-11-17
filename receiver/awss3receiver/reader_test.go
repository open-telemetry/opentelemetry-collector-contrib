// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type readFunc func([]byte) (int, error)

func (f readFunc) Read(p []byte) (int, error) { return f(p) }

func TestReadCancellation(t *testing.T) {
	for _, destination := range []string{"buffer", "discard"} {
		t.Run(destination, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var reads int
			reader := readFunc(func(p []byte) (int, error) {
				reads++
				cancel()
				return copy(p, "data"), nil
			})
			var err error
			if destination == "buffer" {
				_, err = readAll(ctx, reader)
			} else {
				err = copyReader(ctx, io.Discard, reader)
			}
			require.ErrorIs(t, err, context.Canceled)
			require.Equal(t, 1, reads)
		})
	}
}
