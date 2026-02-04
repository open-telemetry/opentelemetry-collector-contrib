// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamUnmarshalOptions(t *testing.T) {
	opts := DecoderOptions{}
	WithFlushBytes(100)(&opts)
	WithFlushItems(50)(&opts)
	WithStreamReaderBuffer(124)(&opts)

	assert.Equal(t, int64(100), opts.FlushBytes)
	assert.Equal(t, int64(50), opts.FlushItems)
	assert.Equal(t, 124, opts.StreamReaderBuffer)
}
