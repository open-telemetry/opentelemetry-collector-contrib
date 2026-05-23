// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"bufio"
	"bytes"
	"errors"
	"io"
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scanner := New(bytes.NewReader(tc.stream), tc.maxSize, make([]byte, 0, tc.bufferSize), tc.startOffset, simpleSplit(tc.delimiter), false)
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
	scanner := New(reader, 100, make([]byte, 0, 100), 0, simpleSplit([]byte("\n")), false)
	assert.False(t, scanner.Scan())
	assert.EqualError(t, scanner.Error(), "log entry too large")

	reader = &errReader{err: errors.New("some err")}
	scanner = New(reader, 100, make([]byte, 0, 100), 0, simpleSplit([]byte("\n")), false)
	assert.False(t, scanner.Scan())
	assert.EqualError(t, scanner.Error(), "scanner error: some err")
}

type oneReadEOFReader struct {
	data []byte
	done bool
}

func (r *oneReadEOFReader) Read(p []byte) (n int, err error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	n = copy(p, r.data)
	// Signal EOF even though we returned data - to force atEOF=true in the scanner split function.
	return n, io.EOF
}

func TestScannerGzip(t *testing.T) {
	testCases := []struct {
		name     string
		reader   io.Reader
		expected [][]byte
		wantPos  int64
	}{
		{
			name:    "flushes unterminated token at EOF",
			reader:  bytes.NewReader([]byte("last-line-without-delimiter")),
			wantPos: int64(len("last-line-without-delimiter")),
			expected: [][]byte{
				[]byte("last-line-without-delimiter"),
			},
		},
		{
			name:    "does not merge tokens (normal reader)",
			reader:  bytes.NewReader([]byte("a\nb\n")),
			wantPos: int64(len("a\nb\n")),
			expected: [][]byte{
				[]byte("a"),
				[]byte("b"),
			},
		},
		{
			name:    "does not merge tokens (atEOF with delimiters)",
			reader:  &oneReadEOFReader{data: []byte("a\nb\n")},
			wantPos: int64(len("a\nb\n")),
			expected: [][]byte{
				[]byte("a"),
				[]byte("b"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s := New(
				tc.reader,
				1024*1024,
				make([]byte, 0, DefaultBufferSize),
				0, simpleSplit([]byte("\n")),
				true,
			)

			var got [][]byte
			for s.Scan() {
				assert.NoError(t, s.Error())
				got = append(got, append([]byte(nil), s.Bytes()...))
			}
			assert.NoError(t, s.Error())

			assert.Equal(t, tc.expected, got)
			assert.Equal(t, tc.wantPos, s.Pos())
		})
	}
}
