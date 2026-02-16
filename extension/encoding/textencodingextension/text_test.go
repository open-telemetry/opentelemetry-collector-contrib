// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
