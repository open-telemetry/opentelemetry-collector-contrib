// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanner(t *testing.T) {
	testCases := []struct {
		name        string
		stream      []byte
		delimiter   []byte
		startOffset int64
		maxSize     int
		expected    [][]byte
	}{
		{
			name:      "simple",
			stream:    []byte("testlog1\ntestlog2\n"),
			delimiter: []byte("\n"),
			maxSize:   100,
			expected: [][]byte{
				[]byte("testlog1"),
				[]byte("testlog2"),
			},
		},
		{
			name:      "empty_tokens",
			stream:    []byte("\ntestlog1\n\ntestlog2\n\n"),
			delimiter: []byte("\n"),
			maxSize:   100,
			expected: [][]byte{
				[]byte(""),
				[]byte("testlog1"),
				[]byte(""),
				[]byte("testlog2"),
				[]byte(""),
			},
		},
		{
			name:      "multichar_delimiter",
			stream:    []byte("testlog1@#$testlog2@#$"),
			delimiter: []byte("@#$"),
			maxSize:   100,
			expected: [][]byte{
				[]byte("testlog1"),
				[]byte("testlog2"),
			},
		},
		{
			name:      "multichar_delimiter_empty_tokens",
			stream:    []byte("@#$testlog1@#$@#$testlog2@#$@#$"),
			delimiter: []byte("@#$"),
			maxSize:   100,
			expected: [][]byte{
				[]byte(""),
				[]byte("testlog1"),
				[]byte(""),
				[]byte("testlog2"),
				[]byte(""),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.stream)
			splitter := simpleSplit(tc.delimiter)
			scanner := NewPositionalScanner(reader, tc.maxSize, tc.startOffset, defaultBufSize, splitter)

			for i, p := 0, 0; scanner.Scan(); i++ {
				require.NoError(t, scanner.getError())

				token := scanner.Bytes()
				require.Equal(t, tc.expected[i], token)

				p += len(tc.expected[i]) + len(tc.delimiter)
				require.Equal(t, int64(p), scanner.Pos())
			}
		})
	}
}

func simpleSplit(delim []byte) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.Index(data, delim); i >= 0 {
			return i + len(delim), data[:i], nil
		}
		return 0, nil, nil
	}
}
