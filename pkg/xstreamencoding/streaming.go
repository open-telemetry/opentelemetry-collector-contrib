// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xstreamencoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// ScannerHelper is a helper to scan new line delimited records from io.Reader and determine when to flush.
// It uses new line delimiters and bytes for batching.
// Not safe for concurrent use.
type ScannerHelper struct {
	batchHelper *BatchHelper
	bufReader   *bufio.Reader
	offset      int64
}

// NewScannerHelper creates a new ScannerHelper that reads from the provided io.Reader.
// It accepts optional encoding.DecoderOption to configure batch flushing behavior.
// If a bufio.Reader is provided, it will be used as-is. Otherwise, one will be derived with default buffer size.
func NewScannerHelper(reader io.Reader, opts ...encoding.DecoderOption) (*ScannerHelper, error) {
	batchHelper := NewBatchHelper(opts...)

	var bufReader *bufio.Reader
	if br, ok := reader.(*bufio.Reader); ok {
		bufReader = br
	} else {
		bufReader = bufio.NewReader(reader)
	}

	if batchHelper.options.Offset != 0 {
		_, err := bufReader.Discard(int(batchHelper.options.Offset))
		if err != nil {
			return nil, fmt.Errorf("failed to discard offset %d: %w", batchHelper.options.Offset, err)
		}
	}

	return &ScannerHelper{
		batchHelper: batchHelper,
		bufReader:   bufReader,
		offset:      batchHelper.options.Offset,
	}, nil
}

// ScanString scans the next line from the stream and returns it as a string. This excludes new line delimiter.
// flush indicates whether the batch should be flushed after processing this string.
// err is non-nil if an error occurred during scanning. If the end of the stream is reached, err will be io.EOF.
func (h *ScannerHelper) ScanString() (line string, flush bool, err error) {
	internal, b, err := h.scanInternal()
	return string(internal), b, err
}

// ScanBytes scans the next line from the stream and returns it as a byte slice. This excludes new line delimiter.
// flush indicates whether the batch should be flushed after processing these bytes.
// err is non-nil if an error occurred during scanning. If the end of the stream is reached, err will be io.EOF.
func (h *ScannerHelper) ScanBytes() (bytes []byte, flush bool, err error) {
	b, flush, err := h.scanInternal()
	if b != nil {
		cpy := make([]byte, len(b))
		copy(cpy, b)
		return cpy, flush, err
	}
	return nil, flush, err
}

func (h *ScannerHelper) scanInternal() ([]byte, bool, error) {
	var isEOF bool
	b, err := h.bufReader.ReadBytes('\n')
	if err != nil {
		if err != io.EOF {
			return nil, false, err
		}
		isEOF = true
	}

	if len(b) == 0 && isEOF {
		return nil, true, io.EOF
	}

	h.offset += int64(len(b))
	h.batchHelper.IncrementBytes(int64(len(b)))
	h.batchHelper.IncrementItems(1)

	var flush bool
	if h.batchHelper.ShouldFlush() {
		h.batchHelper.Reset()
		flush = true
	}

	b = bytes.TrimSpace(b)

	if isEOF {
		return b, flush, io.EOF
	}

	return b, flush, nil
}

// Offset returns the current byte offset read from the stream.
func (h *ScannerHelper) Offset() int64 {
	return h.offset
}

// Options returns the DecoderOptions used by the ScannerHelper's BatchHelper.
func (h *ScannerHelper) Options() encoding.DecoderOptions {
	return h.batchHelper.Options()
}

// BatchHelper is a helper to determine when to flush based on configured options.
// It tracks the current byte and item counts and compares them against configured thresholds.
// Not safe for concurrent use.
type BatchHelper struct {
	options      encoding.DecoderOptions
	currentBytes int64
	currentItems int64
}

// NewBatchHelper creates a new BatchHelper with the provided options.
func NewBatchHelper(opts ...encoding.DecoderOption) *BatchHelper {
	return &BatchHelper{
		options: encoding.NewDecoderOptions(opts...),
	}
}

// IncrementBytes adds n to the current byte count.
func (sh *BatchHelper) IncrementBytes(n int64) {
	sh.currentBytes += n
}

// IncrementItems adds n to the current item count.
func (sh *BatchHelper) IncrementItems(n int64) {
	sh.currentItems += n
}

// ShouldFlush returns true if the current counts exceed configured thresholds.
// Make sure to call Reset after flushing to start tracking the next batch.
func (sh *BatchHelper) ShouldFlush() bool {
	if sh.options.FlushBytes > 0 && sh.currentBytes >= sh.options.FlushBytes {
		return true
	}
	if sh.options.FlushItems > 0 && sh.currentItems >= sh.options.FlushItems {
		return true
	}
	return false
}

// Reset resets the current byte and item counts to zero.
// Should be called after flushing a batch to start tracking the next batch.
func (sh *BatchHelper) Reset() {
	sh.currentBytes = 0
	sh.currentItems = 0
}

// Options returns the DecoderOptions used by the BatchHelper.
func (sh *BatchHelper) Options() encoding.DecoderOptions {
	return sh.options
}

// LogsDecoderAdapter adapts decode and offset functions to implement encoding.LogsDecoder.
type LogsDecoderAdapter struct {
	decode func() (plog.Logs, error)
	offset func() int64
}

// NewLogsDecoderAdapter creates a new LogsDecoderAdapter with the provided decode and offset functions.
func NewLogsDecoderAdapter(decode func() (plog.Logs, error), offset func() int64) LogsDecoderAdapter {
	return LogsDecoderAdapter{
		decode: decode,
		offset: offset,
	}
}

func (a LogsDecoderAdapter) DecodeLogs() (plog.Logs, error) {
	return a.decode()
}

func (a LogsDecoderAdapter) Offset() int64 {
	return a.offset()
}

// MetricsDecoderAdapter adapts decode and offset functions to implement encoding.MetricsDecoder.
type MetricsDecoderAdapter struct {
	decode func() (pmetric.Metrics, error)
	offset func() int64
}

// NewMetricsDecoderAdapter creates a new MetricsDecoderAdapter with the provided decode and offset functions.
func NewMetricsDecoderAdapter(decode func() (pmetric.Metrics, error), offset func() int64) MetricsDecoderAdapter {
	return MetricsDecoderAdapter{
		decode: decode,
		offset: offset,
	}
}

func (a MetricsDecoderAdapter) DecodeMetrics() (pmetric.Metrics, error) {
	return a.decode()
}

func (a MetricsDecoderAdapter) Offset() int64 {
	return a.offset()
}
