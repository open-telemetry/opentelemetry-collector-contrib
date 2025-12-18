// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// NewLogsStreamDecoder creates a new stream decoder for AWS logs
func (e *encodingExtension) NewLogsStreamDecoder(
	ctx context.Context,
	r io.Reader,
	opts ...encoding.StreamDecoderOption,
) (encoding.StreamDecoder[plog.Logs], error) {
	// Parse options
	options := encoding.StreamDecoderOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	// Wrap reader for gzip handling if needed
	reader, err := e.getStreamReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get stream reader: %w", err)
	}

	// Create stream decoder using factory
	decoder, err := e.unmarshalerFactory(reader, options)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream decoder: %w", err)
	}
	return decoder, nil
}

// getStreamReader wraps the reader and handles gzip decompression if needed
func (e *encodingExtension) getStreamReader(r io.Reader) (io.Reader, error) {
	// For streaming, we need to detect gzip by peeking at the first bytes
	// We'll use a peek reader to check magic bytes
	peekReader := &peekReader{reader: r}
	peeked, err := peekReader.Peek(2)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("failed to peek at stream: %w", err)
	}

	if len(peeked) >= 2 && peeked[0] == 0x1f && peeked[1] == 0x8b {
		// Gzip detected
		gzipReader, err := gzip.NewReader(peekReader)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return &gzipStreamReader{reader: gzipReader, pool: &e.gzipPool}, nil
	}

	return peekReader, nil
}

// peekReader allows peeking at the start of a stream
type peekReader struct {
	reader io.Reader
	buffer []byte
	pos    int
}

func (pr *peekReader) Peek(n int) ([]byte, error) {
	if len(pr.buffer) < n {
		read := make([]byte, n-len(pr.buffer))
		m, err := pr.reader.Read(read)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		pr.buffer = append(pr.buffer, read[:m]...)
	}
	if len(pr.buffer) < n {
		return pr.buffer, io.EOF
	}
	return pr.buffer[:n], nil
}

func (pr *peekReader) Read(p []byte) (int, error) {
	if pr.pos < len(pr.buffer) {
		n := copy(p, pr.buffer[pr.pos:])
		pr.pos += n
		return n, nil
	}
	return pr.reader.Read(p)
}

// gzipStreamReader wraps a gzip reader and returns it to the pool on close
type gzipStreamReader struct {
	reader *gzip.Reader
	pool   *sync.Pool
	closed bool
}

func (gsr *gzipStreamReader) Read(p []byte) (int, error) {
	if gsr.closed {
		return 0, io.ErrClosedPipe
	}
	return gsr.reader.Read(p)
}

func (gsr *gzipStreamReader) Close() error {
	if gsr.closed {
		return nil
	}
	gsr.closed = true
	err := gsr.reader.Close()
	gsr.pool.Put(gsr.reader)
	return err
}
