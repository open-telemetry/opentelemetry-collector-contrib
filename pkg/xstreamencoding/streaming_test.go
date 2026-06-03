// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xstreamencoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding"

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

func TestStreamScannerHelper_constructor(t *testing.T) {
	t.Run("IO reader get converted to bufio.Reader", func(t *testing.T) {
		reader := strings.NewReader("test")

		helper, err := NewScannerHelper(reader)
		require.NoError(t, err)

		assert.IsType(t, &bufio.Reader{}, helper.bufReader)
	})

	t.Run("Bufio.Reader remains unchanged", func(t *testing.T) {
		reader := strings.NewReader("test")
		bufReader := bufio.NewReader(reader)

		helper, err := NewScannerHelper(bufReader)
		require.NoError(t, err)

		assert.Equal(t, bufReader, helper.bufReader)
	})

	t.Run("Offset is checked and error wil be returned if incorrect", func(t *testing.T) {
		reader := strings.NewReader("test")
		bufReader := bufio.NewReader(reader)

		_, err := NewScannerHelper(bufReader, encoding.WithOffset(10))
		require.ErrorContains(t, err, "failed to discard offset 10")
	})
}

func TestStreamScannerHelper_ScanString(t *testing.T) {
	input := "line1\nline2\nline3\n"
	helper, err := NewScannerHelper(strings.NewReader(input))
	require.NoError(t, err)

	line, flush, err := helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line1", line)
	assert.False(t, flush)
	require.Equal(t, int64(6), helper.Offset())

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line2", line)
	assert.False(t, flush)
	require.Equal(t, int64(12), helper.Offset())

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line3", line)
	assert.False(t, flush)
	require.Equal(t, int64(18), helper.Offset())

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamScannerHelper_ScanBytes(t *testing.T) {
	input := "line1\nline2\nline3\n"
	helper, err := NewScannerHelper(strings.NewReader(input))
	require.NoError(t, err)

	bytes, flush, err := helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line1"), bytes)
	assert.False(t, flush)
	require.Equal(t, int64(6), helper.Offset())

	bytes, _, err = helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line2"), bytes)
	assert.False(t, flush)
	require.Equal(t, int64(12), helper.Offset())

	bytes, _, err = helper.ScanBytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("line3"), bytes)
	assert.False(t, flush)
	require.Equal(t, int64(18), helper.Offset())

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamScannerHelper_InitialOffset(t *testing.T) {
	input := "line1\nline2\nline3\n"

	// Skip "line1\n" by setting offset to 6
	helper, err := NewScannerHelper(strings.NewReader(input), encoding.WithOffset(6))
	require.NoError(t, err)

	require.Equal(t, int64(6), helper.Offset())

	line, flush, err := helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line2", line)
	assert.False(t, flush)
	require.Equal(t, int64(12), helper.Offset())

	line, flush, err = helper.ScanString()
	require.NoError(t, err)
	assert.Equal(t, "line3", line)
	assert.False(t, flush)
	require.Equal(t, int64(18), helper.Offset())

	_, flush, err = helper.ScanString()
	assert.ErrorIs(t, err, io.EOF)
	assert.True(t, flush)
}

func TestStreamBatchHelper_ShouldFlush(t *testing.T) {
	helper := NewBatchHelper(encoding.WithFlushBytes(5), encoding.WithFlushItems(5))

	assert.False(t, helper.ShouldFlush())

	helper.IncrementBytes(5)
	assert.True(t, helper.ShouldFlush())

	helper.Reset()
	assert.False(t, helper.ShouldFlush())

	helper.IncrementItems(5)
	assert.True(t, helper.ShouldFlush())
}

type stubLogsUnmarshaler struct {
	logs plog.Logs
	err  error
}

func (s *stubLogsUnmarshaler) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return s.logs, s.err
}

type capturingLogsUnmarshaler struct {
	received *[]byte
}

func (s *capturingLogsUnmarshaler) UnmarshalLogs(data []byte) (plog.Logs, error) {
	*s.received = data
	return plog.NewLogs(), nil
}

type stubMetricsUnmarshaler struct {
	metrics pmetric.Metrics
	err     error
}

func (s *stubMetricsUnmarshaler) UnmarshalMetrics(_ []byte) (pmetric.Metrics, error) {
	return s.metrics, s.err
}

type capturingMetricsUnmarshaler struct {
	received *[]byte
}

func (s *capturingMetricsUnmarshaler) UnmarshalMetrics(data []byte) (pmetric.Metrics, error) {
	*s.received = data
	return pmetric.NewMetrics(), nil
}

func TestLogsUnmarshalerDecoderFactory(t *testing.T) {
	t.Run("decode returns result then EOF", func(t *testing.T) {
		expected := plog.NewLogs()
		expected.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("hello")

		factory := NewLogsUnmarshalerDecoderFactory(&stubLogsUnmarshaler{logs: expected})
		input := "some log data"
		decoder, err := factory.NewLogsDecoder(strings.NewReader(input))
		require.NoError(t, err)

		logs, err := decoder.DecodeLogs()
		require.NoError(t, err)
		assert.Equal(t, expected, logs)

		_, err = decoder.DecodeLogs()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("offset equals bytes read", func(t *testing.T) {
		factory := NewLogsUnmarshalerDecoderFactory(&stubLogsUnmarshaler{logs: plog.NewLogs()})
		input := "12345678"
		decoder, err := factory.NewLogsDecoder(strings.NewReader(input))
		require.NoError(t, err)

		assert.Equal(t, int64(0), decoder.Offset())

		_, err = decoder.DecodeLogs()
		require.NoError(t, err)
		assert.Equal(t, int64(len(input)), decoder.Offset())
	})

	t.Run("with offset option skips bytes", func(t *testing.T) {
		input := "XXXXXreal data"
		var received []byte
		captureUnmarshaler := &capturingLogsUnmarshaler{received: &received}
		factory := NewLogsUnmarshalerDecoderFactory(captureUnmarshaler)

		decoder, err := factory.NewLogsDecoder(strings.NewReader(input), encoding.WithOffset(5))
		require.NoError(t, err)

		_, err = decoder.DecodeLogs()
		require.NoError(t, err)
		assert.Equal(t, []byte("real data"), received)
		assert.Equal(t, int64(len(input)), decoder.Offset())
	})

	t.Run("unmarshal error propagates", func(t *testing.T) {
		factory := NewLogsUnmarshalerDecoderFactory(&stubLogsUnmarshaler{err: assert.AnError})
		decoder, err := factory.NewLogsDecoder(strings.NewReader("data"))
		require.NoError(t, err)

		_, err = decoder.DecodeLogs()
		require.ErrorIs(t, err, assert.AnError)
	})
}

func TestMetricsUnmarshalerDecoderFactory(t *testing.T) {
	t.Run("decode returns result then EOF", func(t *testing.T) {
		expected := pmetric.NewMetrics()
		expected.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("test")

		factory := NewMetricsUnmarshalerDecoderFactory(&stubMetricsUnmarshaler{metrics: expected})
		input := "some metric data"
		decoder, err := factory.NewMetricsDecoder(strings.NewReader(input))
		require.NoError(t, err)

		metrics, err := decoder.DecodeMetrics()
		require.NoError(t, err)
		assert.Equal(t, expected, metrics)

		_, err = decoder.DecodeMetrics()
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("offset equals bytes read", func(t *testing.T) {
		factory := NewMetricsUnmarshalerDecoderFactory(&stubMetricsUnmarshaler{metrics: pmetric.NewMetrics()})
		input := "12345678"
		decoder, err := factory.NewMetricsDecoder(strings.NewReader(input))
		require.NoError(t, err)

		assert.Equal(t, int64(0), decoder.Offset())

		_, err = decoder.DecodeMetrics()
		require.NoError(t, err)
		assert.Equal(t, int64(len(input)), decoder.Offset())
	})

	t.Run("with offset option skips bytes", func(t *testing.T) {
		input := "XXXXXreal data"
		var received []byte
		captureUnmarshaler := &capturingMetricsUnmarshaler{received: &received}
		factory := NewMetricsUnmarshalerDecoderFactory(captureUnmarshaler)

		decoder, err := factory.NewMetricsDecoder(strings.NewReader(input), encoding.WithOffset(5))
		require.NoError(t, err)

		_, err = decoder.DecodeMetrics()
		require.NoError(t, err)
		assert.Equal(t, []byte("real data"), received)
		assert.Equal(t, int64(len(input)), decoder.Offset())
	})

	t.Run("unmarshal error propagates", func(t *testing.T) {
		factory := NewMetricsUnmarshalerDecoderFactory(&stubMetricsUnmarshaler{err: assert.AnError})
		decoder, err := factory.NewMetricsDecoder(strings.NewReader("data"))
		require.NoError(t, err)

		_, err = decoder.DecodeMetrics()
		require.ErrorIs(t, err, assert.AnError)
	})
}
