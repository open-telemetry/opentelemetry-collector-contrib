// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stream // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/stream"

import (
	"bufio"
	"io"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// ScannerHelper is a helper to scan new line delimited records from io.Reader and determine when to flush.
// It wraps a bufio.Scanner and tracks batch metrics to support flushing based on configured options.
// Not safe for concurrent use.
type ScannerHelper struct {
	batchHelper *BatchHelper
	scanner     *bufio.Scanner
}

func NewScannerHelper(reader io.Reader, opts ...encoding.DecoderOption) *ScannerHelper {
	batchHelper := NewBatchHelper(opts...)

	scanner := bufio.NewScanner(reader)
	if batchHelper.options.StreamReaderBuffer > 0 {
		bufSize := batchHelper.options.StreamReaderBuffer
		scanner.Buffer(make([]byte, bufSize), bufSize)
	}

	return &ScannerHelper{
		batchHelper: batchHelper,
		scanner:     scanner,
	}
}

// ScanString scans the next line from the stream and returns it as a string.
// flush indicates whether the batch should be flushed after processing this string.
// err is non-nil if an error occurred during scanning. If the end of the stream is reached, err will be io.EOF.
func (h *ScannerHelper) ScanString() (line string, flush bool, err error) {
	internal, b, err := h.scanInternal()
	return string(internal), b, err
}

// ScanBytes scans the next line from the stream and returns it as a byte slice.
// flush indicates whether the batch should be flushed after processing this these bytes.
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

// BatchHelper is a helper to determine when to flush based on configured options.
// It tracks the current byte and item counts and compares them against configured thresholds.
// Not safe for concurrent use.
type BatchHelper struct {
	options      encoding.DecoderOptions
	currentBytes int64
	currentItems int64
}

func NewBatchHelper(opts ...encoding.DecoderOption) *BatchHelper {
	options := encoding.DecoderOptions{}
	for _, o := range opts {
		o(&options)
	}
	return &BatchHelper{
		options: options,
	}
}

func (sh *BatchHelper) IncrementBytes(n int64) {
	sh.currentBytes += n
}

func (sh *BatchHelper) IncrementItems(n int64) {
	sh.currentItems += n
}

func (sh *BatchHelper) ShouldFlush() bool {
	if sh.options.FlushBytes > 0 && sh.currentBytes >= sh.options.FlushBytes {
		return true
	}
	if sh.options.FlushItems > 0 && sh.currentItems >= sh.options.FlushItems {
		return true
	}
	return false
}

func (sh *BatchHelper) Reset() {
	sh.currentBytes = 0
	sh.currentItems = 0
}

// LogsUnmarshalerFunc is a helper to implement LogsDecoder interface with a wrapper function.
type LogsUnmarshalerFunc struct {
	batchUnmarshal func() (plog.Logs, error)
}

func NewLogsUnmarshalerFunc(batchUnmarshal func() (plog.Logs, error)) *LogsUnmarshalerFunc {
	return &LogsUnmarshalerFunc{
		batchUnmarshal: batchUnmarshal,
	}
}

func (l *LogsUnmarshalerFunc) DecodeLogs() (plog.Logs, error) {
	return l.batchUnmarshal()
}

// MetricsUnmarshalerFunc is a helper to implement MetricsDecoder interface with a wrapper function.
type MetricsUnmarshalerFunc struct {
	batchUnmarshal func() (pmetric.Metrics, error)
}

func NewMetricsUnmarshalerFunc(batchUnmarshal func() (pmetric.Metrics, error)) *MetricsUnmarshalerFunc {
	return &MetricsUnmarshalerFunc{
		batchUnmarshal: batchUnmarshal,
	}
}

func (m *MetricsUnmarshalerFunc) DecodeMetrics() (pmetric.Metrics, error) {
	return m.batchUnmarshal()
}
