// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stream // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/stream"

import (
	"bufio"
	"bytes"
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
	bufReader   *bufio.Reader
}

func NewScannerHelper(reader io.Reader, opts ...encoding.DecoderOption) *ScannerHelper {
	batchHelper := NewBatchHelper(opts...)

	var bufReader *bufio.Reader
	if br, ok := reader.(*bufio.Reader); ok {
		bufReader = br
	} else {
		bufReader = bufio.NewReader(reader)
	}

	return &ScannerHelper{
		batchHelper: batchHelper,
		bufReader:   bufReader,
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

	h.batchHelper.IncrementItems(1)
	h.batchHelper.IncrementBytes(int64(len(b)))

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

// LogsDecoderFunc is a helper to implement encoding.LogsDecoder interface with a wrapper function.
type LogsDecoderFunc struct {
	batchUnmarshal func() (plog.Logs, error)
}

func NewLogsDecoderFunc(batchUnmarshal func() (plog.Logs, error)) *LogsDecoderFunc {
	return &LogsDecoderFunc{
		batchUnmarshal: batchUnmarshal,
	}
}

func (l *LogsDecoderFunc) DecodeLogs() (plog.Logs, error) {
	return l.batchUnmarshal()
}

// MetricsDecoderFunc is a helper to implement encoding.MetricsDecoder interface with a wrapper function.
type MetricsDecoderFunc struct {
	batchUnmarshal func() (pmetric.Metrics, error)
}

func NewMetricsDecoderFunc(batchUnmarshal func() (pmetric.Metrics, error)) *MetricsDecoderFunc {
	return &MetricsDecoderFunc{
		batchUnmarshal: batchUnmarshal,
	}
}

func (m *MetricsDecoderFunc) DecodeMetrics() (pmetric.Metrics, error) {
	return m.batchUnmarshal()
}
