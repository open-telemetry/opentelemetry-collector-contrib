// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecoderOptions(t *testing.T) {
	t.Run("Check Defaults", func(t *testing.T) {
		opts := NewDecoderOptions()

		assert.Equal(t, int64(defaultFlushBytes), opts.FlushBytes)
		assert.Equal(t, int64(defaultFlushItems), opts.FlushItems)
		assert.Equal(t, int64(0), opts.Offset)
	})

	t.Run("Check overrides", func(t *testing.T) {
		opts := NewDecoderOptions()
		WithFlushBytes(100)(&opts)
		WithFlushItems(50)(&opts)
		WithOffset(50)(&opts)

		assert.Equal(t, int64(100), opts.FlushBytes)
		assert.Equal(t, int64(50), opts.FlushItems)
		assert.Equal(t, int64(50), opts.Offset)
	})
}
