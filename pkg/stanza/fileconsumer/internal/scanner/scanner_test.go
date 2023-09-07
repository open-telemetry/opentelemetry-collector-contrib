// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"bufio"
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanner(t *testing.T) {
	testCases := []struct {
		name               string
		stream             []byte
		delimiter          []byte
		startOffset        int64
		maxSize            int
		bufferSize         int
		expected           [][]byte
		skipFirstDelimiter bool
	}{
		{
			name:       "simple",
			stream:     []byte("testlog1\ntestlog2\n"),
			delimiter:  []byte("\n"),
			maxSize:    100,
			bufferSize: DefaultBufferSize,
			expected: [][]byte{
				[]byte("testlog1"),
				[]byte("testlog2"),
			},
		},
		{
			name:       "empty_tokens",
			stream:     []byte("\ntestlog1\n\ntestlog2\n\n"),
			delimiter:  []byte("\n"),
			maxSize:    100,
			bufferSize: DefaultBufferSize,
			expected: [][]byte{
				[]byte(""),
				[]byte("testlog1"),
				[]byte(""),
				[]byte("testlog2"),
				[]byte(""),
			},
		},
		{
			name:       "multichar_delimiter",
			stream:     []byte("testlog1@#$testlog2@#$"),
			delimiter:  []byte("@#$"),
			maxSize:    100,
			bufferSize: DefaultBufferSize,
			expected: [][]byte{
				[]byte("testlog1"),
				[]byte("testlog2"),
			},
		},
		{
			name:       "multichar_delimiter_empty_tokens",
			stream:     []byte("@#$testlog1@#$@#$testlog2@#$@#$"),
			delimiter:  []byte("@#$"),
			maxSize:    100,
			bufferSize: DefaultBufferSize,
			expected: [][]byte{
				[]byte(""),
				[]byte("testlog1"),
				[]byte(""),
				[]byte("testlog2"),
				[]byte(""),
			},
		},
		{
			name:       "overflow_maxlogsize",
			stream:     []byte("testlog1islongerthanmaxlogsize\n"),
			delimiter:  []byte("\n"),
			maxSize:    20,
			bufferSize: DefaultBufferSize,
			expected: [][]byte{
				[]byte("testlog1islongerthan"),
				[]byte("maxlogsize"),
			},
			skipFirstDelimiter: true,
		},
		{
			name:       "overflow_buffer",
			stream:     []byte("testlog1islongerthanbuffer\n"),
			delimiter:  []byte("\n"),
			maxSize:    20,
			bufferSize: 20,
			expected: [][]byte{
				[]byte("testlog1islongerthan"),
				[]byte("buffer"),
			},
			skipFirstDelimiter: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := New(bytes.NewReader(tc.stream), tc.maxSize, tc.bufferSize, tc.startOffset, simpleSplit(tc.delimiter))
			for i, p := 0, 0; scanner.Scan(); i++ {
				assert.NoError(t, scanner.Error())

				token := scanner.Bytes()
				assert.Equal(t, tc.expected[i], token)

				p += len(tc.expected[i])
				if i > 0 || !tc.skipFirstDelimiter {
					p += len(tc.delimiter)
				}
				assert.Equal(t, int64(p), scanner.Pos())
			}
			assert.NoError(t, scanner.Error())
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

type errReader struct {
	err error
}

func (r *errReader) Read([]byte) (n int, err error) {
	return 0, r.err
}

func TestScannerError(t *testing.T) {
	reader := &errReader{err: bufio.ErrTooLong}
	scanner := New(reader, 100, 100, 0, simpleSplit([]byte("\n")))
	assert.False(t, scanner.Scan())
	assert.EqualError(t, scanner.Error(), "log entry too large")

	reader = &errReader{err: errors.New("some err")}
	scanner = New(reader, 100, 100, 0, simpleSplit([]byte("\n")))
	assert.False(t, scanner.Scan())
	assert.EqualError(t, scanner.Error(), "scanner error: some err")
}
