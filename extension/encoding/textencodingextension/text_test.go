// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension

import (
	"bytes"
	"io"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

func TestTextRoundtrip(t *testing.T) {
	enc, err := textutils.LookupEncoding("utf8")
	require.NoError(t, err)
	r := regexp.MustCompile(`\r?\n`)
	codec := &textLogCodec{decoder: enc.NewDecoder(), unmarshalingSeparator: r, marshalingSeparator: "\n"}
	require.NoError(t, err)
	ld, err := codec.UnmarshalLogs([]byte("foo\r\nbar\n"))
	require.NoError(t, err)
	assert.Equal(t, 2, ld.LogRecordCount())
	b, err := codec.MarshalLogs(ld)
	require.NoError(t, err)
	require.Equal(t, "foo\nbar", string(b))
}

func TestTextRoundtripMissingNewline(t *testing.T) {
	enc, err := textutils.LookupEncoding("utf8")
	require.NoError(t, err)
	r := regexp.MustCompile(`\r?\n`)
	codec := &textLogCodec{decoder: enc.NewDecoder(), unmarshalingSeparator: r, marshalingSeparator: "\n"}
	require.NoError(t, err)
	ld, err := codec.UnmarshalLogs([]byte("foo\r\nbar"))
	require.NoError(t, err)
	assert.Equal(t, 2, ld.LogRecordCount())
	b, err := codec.MarshalLogs(ld)
	require.NoError(t, err)
	require.Equal(t, "foo\nbar", string(b))
}

func TestNoSeparator(t *testing.T) {
	enc, err := textutils.LookupEncoding("utf8")
	require.NoError(t, err)
	codec := &textLogCodec{decoder: enc.NewDecoder()}
	require.NoError(t, err)
	ld, err := codec.UnmarshalLogs([]byte("foo\r\nbar\n"))
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	b, err := codec.MarshalLogs(ld)
	require.NoError(t, err)
	require.Equal(t, "foo\r\nbar\n", string(b))
}

func TestNoSeparatorLargeMessage(t *testing.T) {
	// Test that large messages (> bufio.Scanner's default 4096 buffer) are not split
	// when no separator is configured. This verifies the fix for the bug where the
	// split function would return immediately without waiting for EOF.
	enc, err := textutils.LookupEncoding("utf8")
	require.NoError(t, err)
	codec := &textLogCodec{decoder: enc.NewDecoder()}

	// Create a message larger than bufio.Scanner's default 4096 byte buffer
	largeMessage := make([]byte, 16384)
	for i := range largeMessage {
		largeMessage[i] = byte('a' + (i % 26))
	}

	ld, err := codec.UnmarshalLogs(largeMessage)
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount(), "large message should not be split into multiple records")

	b, err := codec.MarshalLogs(ld)
	require.NoError(t, err)
	require.Equal(t, largeMessage, b)
}

func TestStreamDecoding_singleFlush(t *testing.T) {
	enc, err := textutils.LookupEncoding("utf8")
	require.NoError(t, err)
	r := regexp.MustCompile(`\r?\n`)
	codec := &textLogCodec{decoder: enc.NewDecoder(), unmarshalingSeparator: r, marshalingSeparator: "\n"}

	reader := bytes.NewReader([]byte("foo\nbar\nbaz\n"))

	// Decode with WithFlushItems=1
	decoder, err := codec.NewLogsDecoder(reader, encoding.WithFlushItems(1))
	require.NoError(t, err)

	// First call should return "foo"
	ld, err := decoder.DecodeLogs()
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	assert.Equal(t, "foo", ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, int64(4), decoder.Offset())

	// Second call should return "bar"
	ld, err = decoder.DecodeLogs()
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	assert.Equal(t, "bar", ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, int64(8), decoder.Offset())

	// Third call should return "baz"
	ld, err = decoder.DecodeLogs()
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	assert.Equal(t, "baz", ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, int64(12), decoder.Offset())

	// Fourth call should return EOF with empty logs
	ld, err = decoder.DecodeLogs()
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, 0, ld.LogRecordCount())
}

func TestStreamDecoding_offsetWithFlush(t *testing.T) {
	enc, err := textutils.LookupEncoding("utf8")
	require.NoError(t, err)
	r := regexp.MustCompile(`\r?\n`)
	codec := &textLogCodec{decoder: enc.NewDecoder(), unmarshalingSeparator: r, marshalingSeparator: "\n"}

	reader := bytes.NewReader([]byte("foo\nbar\nbaz\n"))

	// Decode with offset=1 to skip "foo", flush after each item
	decoder, err := codec.NewLogsDecoder(reader, encoding.WithOffset(4), encoding.WithFlushItems(1))
	require.NoError(t, err)

	// First call should return "bar" (skipped "foo")
	ld, err := decoder.DecodeLogs()
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	assert.Equal(t, "bar", ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, int64(8), decoder.Offset())

	// Second call should return "baz"
	ld, err = decoder.DecodeLogs()
	require.NoError(t, err)
	assert.Equal(t, 1, ld.LogRecordCount())
	assert.Equal(t, "baz", ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, int64(12), decoder.Offset())

	// Third call should return EOF with empty logs
	ld, err = decoder.DecodeLogs()
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, 0, ld.LogRecordCount())
}

func TestStreamDecoding_flushAll(t *testing.T) {
	enc, err := textutils.LookupEncoding("utf8")
	require.NoError(t, err)
	r := regexp.MustCompile(`\r?\n`)
	codec := &textLogCodec{decoder: enc.NewDecoder(), unmarshalingSeparator: r, marshalingSeparator: "\n"}

	reader := bytes.NewReader([]byte("foo\nbar\nbaz\n"))

	// Decode with zero flush options to flush all records at once
	decoder, err := codec.NewLogsDecoder(reader, encoding.WithFlushItems(0), encoding.WithFlushBytes(0))
	require.NoError(t, err)

	// First call should return all 3 records (each record in separate ResourceLogs)
	ld, err := decoder.DecodeLogs()
	require.NoError(t, err)
	assert.Equal(t, 3, ld.LogRecordCount())
	assert.Equal(t, "foo", ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, "bar", ld.ResourceLogs().At(1).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())
	assert.Equal(t, "baz", ld.ResourceLogs().At(2).ScopeLogs().At(0).LogRecords().At(0).Body().AsString())

	// Second call should return EOF with empty logs
	ld, err = decoder.DecodeLogs()
	assert.ErrorIs(t, err, io.EOF)
	assert.Equal(t, 0, ld.LogRecordCount())
}
