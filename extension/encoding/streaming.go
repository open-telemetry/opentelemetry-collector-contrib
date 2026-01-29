// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"bufio"
	"context"
	"io"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// StreamUnmarshalOptions configures the behavior of stream unmarshaling.
type StreamUnmarshalOptions struct {
	FlushBytes         int64
	FlushItems         int64
	StreamReaderBuffer int
}

// StreamUnmarshalOption defines the functional option for StreamUnmarshalOptions.
type StreamUnmarshalOption func(*StreamUnmarshalOptions)

// WithFlushBytes sets the number of bytes after stream unmarshaler should flush.
func WithFlushBytes(b int64) StreamUnmarshalOption {
	return func(o *StreamUnmarshalOptions) {
		o.FlushBytes = b
	}
}

// WithFlushItems sets the number of items after stream unmarshaler should flush.
func WithFlushItems(i int64) StreamUnmarshalOption {
	return func(o *StreamUnmarshalOptions) {
		o.FlushItems = i
	}
}

// WithStreamReaderBuffer sets the size of buffer that should be used by the stream reader.
func WithStreamReaderBuffer(size int) StreamUnmarshalOption {
	return func(o *StreamUnmarshalOptions) {
		o.StreamReaderBuffer = size
	}
}

// StreamScannerHelper is a helper to scan new line delimited records from io.Reader and determine when to flush.
// It wraps a bufio.Scanner and tracks batch metrics to support flushing based on configured options.
// Not safe for concurrent use.
type StreamScannerHelper struct {
	batchHelper *StreamBatchHelper
	scanner     *bufio.Scanner
}

func NewStreamScannerHelper(reader io.Reader, opts ...StreamUnmarshalOption) *StreamScannerHelper {
	batchHelper := NewStreamBatchHelper(opts...)

	scanner := bufio.NewScanner(reader)
	if batchHelper.options.StreamReaderBuffer > 0 {
		bufSize := batchHelper.options.StreamReaderBuffer
		scanner.Buffer(make([]byte, bufSize), bufSize)
	}

	return &StreamScannerHelper{
		batchHelper: batchHelper,
		scanner:     scanner,
	}
}

// ScanString scans the next line from the stream and returns it as a string.
// flush indicates whether the batch should be flushed after processing this string.
// err is non-nil if an error occurred during scanning. If the end of the stream is reached, err will be io.EOF.
func (h *StreamScannerHelper) ScanString() (line string, flush bool, err error) {
	internal, b, err := h.scanInternal()
	return string(internal), b, err
}

// ScanBytes scans the next line from the stream and returns it as a byte slice.
// flush indicates whether the batch should be flushed after processing this these bytes.
// err is non-nil if an error occurred during scanning. If the end of the stream is reached, err will be io.EOF.
func (h *StreamScannerHelper) ScanBytes() (bytes []byte, flush bool, err error) {
	b, flush, err := h.scanInternal()
	if b != nil {
		cpy := make([]byte, len(b))
		copy(cpy, b)
		return cpy, flush, err
	}
	return nil, flush, err
}

func (h *StreamScannerHelper) scanInternal() ([]byte, bool, error) {
	if !h.scanner.Scan() {
		if err := h.scanner.Err(); err != nil {
			return nil, true, err
		}
		return nil, true, io.EOF
	}

	b := h.scanner.Bytes()
	h.batchHelper.IncrementItems(1)
	// +1 for the newline character that was scanned but not included in b
	h.batchHelper.IncrementBytes(int64(len(b)) + 1)

	if h.batchHelper.ShouldFlush() {
		h.batchHelper.Reset()

		return b, true, nil
	}

	return b, false, nil
}

// StreamBatchHelper is a helper to determine when to flush based on configured options.
// It tracks the current byte and item counts and compares them against configured thresholds.
// Not safe for concurrent use.
type StreamBatchHelper struct {
	options      StreamUnmarshalOptions
	currentBytes int64
	currentItems int64
}

func NewStreamBatchHelper(opts ...StreamUnmarshalOption) *StreamBatchHelper {
	options := StreamUnmarshalOptions{}
	for _, o := range opts {
		o(&options)
	}
	return &StreamBatchHelper{
		options: options,
	}
}

func (sh *StreamBatchHelper) IncrementBytes(n int64) {
	sh.currentBytes += n
}

func (sh *StreamBatchHelper) IncrementItems(n int64) {
	sh.currentItems += n
}

func (sh *StreamBatchHelper) ShouldFlush() bool {
	if sh.options.FlushBytes > 0 && sh.currentBytes >= sh.options.FlushBytes {
		return true
	}
	if sh.options.FlushItems > 0 && sh.currentItems >= sh.options.FlushItems {
		return true
	}
	return false
}

func (sh *StreamBatchHelper) Reset() {
	sh.currentBytes = 0
	sh.currentItems = 0
}

// LogsStreamUnmarshalerFunc is a helper to implement LogsStreamUnmarshaler interface with a wrapper function.
type LogsStreamUnmarshalerFunc struct {
	batchUnmarshal func(context.Context) (plog.Logs, error)
}

func NewLogsStreamUnmarshalerFunc(batchUnmarshal func(context.Context) (plog.Logs, error)) *LogsStreamUnmarshalerFunc {
	return &LogsStreamUnmarshalerFunc{
		batchUnmarshal: batchUnmarshal,
	}
}

func (l *LogsStreamUnmarshalerFunc) UnmarshalBatch(ctx context.Context) (plog.Logs, error) {
	return l.batchUnmarshal(ctx)
}

// MetricsStreamUnmarshalerFunc is a helper to implement MetricsStreamUnmarshaler interface with a wrapper function.
type MetricsStreamUnmarshalerFunc struct {
	batchUnmarshal func(context.Context) (pmetric.Metrics, error)
}

func NewMetricsStreamUnmarshalerFunc(batchUnmarshal func(context.Context) (pmetric.Metrics, error)) *MetricsStreamUnmarshalerFunc {
	return &MetricsStreamUnmarshalerFunc{
		batchUnmarshal: batchUnmarshal,
	}
}

func (m *MetricsStreamUnmarshalerFunc) UnmarshalBatch(ctx context.Context) (pmetric.Metrics, error) {
	return m.batchUnmarshal(ctx)
}
