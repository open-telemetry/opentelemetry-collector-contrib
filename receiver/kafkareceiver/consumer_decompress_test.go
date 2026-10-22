// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestAsDecompressTooLarge(t *testing.T) {
	decompressErr := &kgo.ErrDecompressTooLarge{
		Topic:      "otlp_spans",
		Partition:  3,
		Offset:     100,
		Epoch:      7,
		NextOffset: 142,
	}

	got, ok := asDecompressTooLarge(decompressErr)
	require.True(t, ok)
	assert.Equal(t, "otlp_spans", got.Topic)
	assert.Equal(t, int32(3), got.Partition)
	assert.Equal(t, int64(100), got.Offset)
	assert.Equal(t, int32(7), got.Epoch)
	assert.Equal(t, int64(142), got.NextOffset)

	// Wrapped errors must still be recognised (franz-go may wrap the error).
	got, ok = asDecompressTooLarge(fmt.Errorf("fetch failed: %w", decompressErr))
	require.True(t, ok)
	assert.Equal(t, int64(142), got.NextOffset)

	_, ok = asDecompressTooLarge(errors.New("some other fetch error"))
	assert.False(t, ok)

	_, ok = asDecompressTooLarge(nil)
	assert.False(t, ok)
}
