// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter/internal"

import (
	"bytes"
	"encoding/json"
)

// Encoder is an interface for encoding data
type Encoder interface {
	Encode(v any) error
}

// ChunkedEncoder is an encoder that splits the encoded data into chunks
// when the total size exceeds a configured maximum.
type ChunkedEncoder struct {
	maxSize       int
	currentBuffer *bytes.Buffer
	buffers       []*bytes.Buffer
}

// NewChunkedEncoderWithMaxSize creates a new ChunkedEncoder with the specified maximum size.
func NewChunkedEncoder(maxChunkSize int) *ChunkedEncoder {
	return &ChunkedEncoder{
		maxSize:       maxChunkSize,
		currentBuffer: nil,
		buffers:       make([]*bytes.Buffer, 0, 1),
	}
}

// Encode encodes the value to the current buffer.
// If encoding the value would exceed the maximum size,
// a new buffer is created and the value is encoded to the new buffer.
func (e *ChunkedEncoder) Encode(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	if e.currentBuffer == nil || e.currentBuffer.Len()+len(data)+1 >= e.maxSize {
		e.currentBuffer = bytes.NewBuffer(nil)
		e.buffers = append(e.buffers, e.currentBuffer)
	}

	e.currentBuffer.Write(data)
	e.currentBuffer.WriteByte('\n')

	return nil
}

func (e *ChunkedEncoder) Buffers() []*bytes.Buffer {
	return e.buffers
}
