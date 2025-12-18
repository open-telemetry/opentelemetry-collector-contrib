// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"context"
	"time"
)

type StreamDecoderFactory[T any] interface {
	NewStreamDecoder(ctx context.Context, opts ...StreamDecoderOption) (StreamDecoder[T], error)
}

type StreamDecoder[T any] interface {
	// Decode reads the next batch from the stream into "to".
	//
	// Decode returns io.EOF when the end of the stream is reached.
	Decode(ctx context.Context, to T) error

	// Offset returns the current offset in the stream, immediately
	// after the last read. The initial offset will be 0.
	Offset() StreamOffset
}

// StreamOffset represents a position in the stream. The meaning of the
// offset is defined by the stream decoder, and is not necessarily counted
// in bytes.
type StreamOffset int64

type StreamDecoderOptions struct {
	InitialOffset StreamOffset
	FlushBytes    int64
	FlushItems    int64
	FlushTimeout  time.Duration
}

type StreamDecoderOption func(*StreamDecoderOptions)

// WithInitialOffset sets the initial offset for the stream decoder,
// where the offset is defined by the stream decoder.
//
// This can be used to resume decoding from a specific position in the
// stream. This function must only be called with offsets previously
// obtained from the Offset() method of the same type of stream decoder.
//
// By default, decoding starts from the beginning of the stream.
func WithInitialOffset(offset StreamOffset) StreamDecoderOption {
	return func(opts *StreamDecoderOptions) {
		opts.InitialOffset = offset
	}
}

func WithFlushBytes(bytes int64) StreamDecoderOption {
	return func(opts *StreamDecoderOptions) {
		opts.FlushBytes = bytes
	}
}

func WithFlushItems(items int64) StreamDecoderOption {
	return func(opts *StreamDecoderOptions) {
		opts.FlushItems = items
	}
}

func WithFlushTimeout(timeout time.Duration) StreamDecoderOption {
	return func(opts *StreamDecoderOptions) {
		opts.FlushTimeout = timeout
	}
}
