// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
)

func TestReaderUpdateFingerprint(t *testing.T) {
	bufferSizes := []int{2, 3, 5, 8, 10, 13, 20, 50}
	testCases := []updateFingerprintTest{
		{
			name:              "new_file",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte(""),
			moreBytes:         []byte("1234567890\n"),
			expectTokens:      [][]byte{[]byte("1234567890")},
			expectOffset:      11,
			expectFingerprint: []byte("1234567890"),
		},
		{
			name:              "existing_partial_line_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("1234567890\n"),
			expectTokens:      [][]byte{[]byte("foo1234567890")},
			expectOffset:      14,
			expectFingerprint: []byte("foo1234567"),
		},
		{
			name:              "existing_partial_line",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("1234567890\n"),
			expectTokens:      [][]byte{[]byte("1234567890")},
			expectOffset:      14,
			expectFingerprint: []byte("foo1234567"),
		},
		{
			name:              "existing_full_line_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte("foo\n"),
			moreBytes:         []byte("1234567890\n"),
			expectTokens:      [][]byte{[]byte("foo"), []byte("1234567890")},
			expectOffset:      15,
			expectFingerprint: []byte("foo\n123456"),
		},
		{
			name:              "existing_full_line",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte("foo\n"),
			moreBytes:         []byte("1234567890\n"),
			expectTokens:      [][]byte{[]byte("1234567890")},
			expectOffset:      15,
			expectFingerprint: []byte("foo\n123456"),
		},
		{
			name:              "split_none_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("1234567890"),
			expectTokens:      [][]byte{},
			expectOffset:      0,
			expectFingerprint: []byte("foo1234567"),
		},
		{
			name:              "split_none",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("1234567890"),
			expectTokens:      [][]byte{},
			expectOffset:      3,
			expectFingerprint: []byte("foo1234567"),
		},
		{
			name:              "split_mid_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("12345\n67890"),
			expectTokens:      [][]byte{[]byte("foo12345")},
			expectOffset:      9,
			expectFingerprint: []byte("foo12345\n6"),
		},
		{
			name:              "split_mid",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("12345\n67890"),
			expectTokens:      [][]byte{[]byte("12345")},
			expectOffset:      9,
			expectFingerprint: []byte("foo12345\n6"),
		},
		{
			name:              "clean_end_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("12345\n67890\n"),
			expectTokens:      [][]byte{[]byte("foo12345"), []byte("67890")},
			expectOffset:      15,
			expectFingerprint: []byte("foo12345\n6"),
		},
		{
			name:              "clean_end",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("12345\n67890\n"),
			expectTokens:      [][]byte{[]byte("12345"), []byte("67890")},
			expectOffset:      15,
			expectFingerprint: []byte("foo12345\n6"),
		},
		{
			name:              "full_lines_only_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte("foo\n"),
			moreBytes:         []byte("12345\n67890\n"),
			expectTokens:      [][]byte{[]byte("foo"), []byte("12345"), []byte("67890")},
			expectOffset:      16,
			expectFingerprint: []byte("foo\n12345\n"),
		},
		{
			name:              "full_lines_only",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte("foo\n"),
			moreBytes:         []byte("12345\n67890\n"),
			expectTokens:      [][]byte{[]byte("12345"), []byte("67890")},
			expectOffset:      16,
			expectFingerprint: []byte("foo\n12345\n"),
		},
		{
			name:              "small_max_log_size_from_start",
			fingerprintSize:   20,
			maxLogSize:        4,
			fromBeginning:     true,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("1234567890\nbar\nhelloworld\n"),
			expectTokens:      [][]byte{[]byte("foo1"), []byte("2345"), []byte("6789"), []byte("0"), []byte("bar"), []byte("hell"), []byte("owor"), []byte("ld")},
			expectOffset:      29,
			expectFingerprint: []byte("foo1234567890\nbar\nhe"),
		},
		{
			name:              "small_max_log_size",
			fingerprintSize:   20,
			maxLogSize:        4,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("1234567890\nbar\nhelloworld\n"),
			expectTokens:      [][]byte{[]byte("1234"), []byte("5678"), []byte("90"), []byte("bar"), []byte("hell"), []byte("owor"), []byte("ld")},
			expectOffset:      29,
			expectFingerprint: []byte("foo1234567890\nbar\nhe"),
		},
		{
			name:              "leading_empty_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte(""),
			moreBytes:         []byte("\n12345\n67890\n"),
			expectTokens:      [][]byte{[]byte(""), []byte("12345"), []byte("67890")},
			expectOffset:      13,
			expectFingerprint: []byte("\n12345\n678"),
		},
		{
			name:              "leading_empty",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte(""),
			moreBytes:         []byte("\n12345\n67890\n"),
			expectTokens:      [][]byte{[]byte(""), []byte("12345"), []byte("67890")},
			expectOffset:      13,
			expectFingerprint: []byte("\n12345\n678"),
		},
		{
			name:              "multiple_empty_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte(""),
			moreBytes:         []byte("\n\n12345\n\n67890\n\n"),
			expectTokens:      [][]byte{[]byte(""), []byte(""), []byte("12345"), []byte(""), []byte("67890"), []byte("")},
			expectOffset:      16,
			expectFingerprint: []byte("\n\n12345\n\n6"),
		},
		{
			name:              "multiple_empty",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte(""),
			moreBytes:         []byte("\n\n12345\n\n67890\n\n"),
			expectTokens:      [][]byte{[]byte(""), []byte(""), []byte("12345"), []byte(""), []byte("67890"), []byte("")},
			expectOffset:      16,
			expectFingerprint: []byte("\n\n12345\n\n6"),
		},
		{
			name:              "multiple_empty_partial_end_from_start",
			fingerprintSize:   10,
			maxLogSize:        100,
			fromBeginning:     true,
			initBytes:         []byte(""),
			moreBytes:         []byte("\n\n12345\n\n67890"),
			expectTokens:      [][]byte{[]byte(""), []byte(""), []byte("12345"), []byte("")},
			expectOffset:      9,
			expectFingerprint: []byte("\n\n12345\n\n6"),
		},
		{
			name:              "multiple_empty_partial_end",
			fingerprintSize:   10,
			maxLogSize:        100,
			initBytes:         []byte(""),
			moreBytes:         []byte("\n\n12345\n\n67890"),
			expectTokens:      [][]byte{[]byte(""), []byte(""), []byte("12345"), []byte("")},
			expectOffset:      9,
			expectFingerprint: []byte("\n\n12345\n\n6"),
		},
	}

	for _, tc := range testCases {
		for _, bufferSize := range bufferSizes {
			t.Run(fmt.Sprintf("%s/bufferSize:%d", tc.name, bufferSize), tc.run(bufferSize))
		}
	}
}

type updateFingerprintTest struct {
	name              string
	fingerprintSize   int
	maxLogSize        int
	fromBeginning     bool
	initBytes         []byte
	moreBytes         []byte
	expectTokens      [][]byte
	expectOffset      int64
	expectFingerprint []byte
}

func (tc updateFingerprintTest) run(bufferSize int) func(*testing.T) {
	return func(t *testing.T) {
		opts := []testFactoryOpt{
			withFingerprintSize(tc.fingerprintSize),
			withInitialBufferSize(bufferSize),
			withMaxLogSize(tc.maxLogSize),
			withFlushPeriod(0),
		}
		if !tc.fromBeginning {
			opts = append(opts, fromEnd())
		}
		f, sink := testFactory(t, opts...)

		temp := filetest.OpenTemp(t, t.TempDir())
		_, err := temp.Write(tc.initBytes)
		require.NoError(t, err)

		fi, err := temp.Stat()
		require.NoError(t, err)
		require.Equal(t, int64(len(tc.initBytes)), fi.Size())

		fp, err := f.NewFingerprint(temp)
		require.NoError(t, err)
		r, err := f.NewReader(temp, fp)
		require.NoError(t, err)
		require.Same(t, temp, r.file)

		if tc.fromBeginning {
			assert.Equal(t, int64(0), r.Offset)
		} else {
			assert.Equal(t, int64(len(tc.initBytes)), r.Offset)
		}
		assert.Equal(t, fingerprint.New(tc.initBytes), r.Fingerprint)

		i, err := temp.Write(tc.moreBytes)
		require.NoError(t, err)
		require.Len(t, tc.moreBytes, i)

		r.ReadToEnd(context.Background())

		sink.ExpectTokens(t, tc.expectTokens...)

		assert.Equal(t, tc.expectOffset, r.Offset)
		assert.Equal(t, fingerprint.New(tc.expectFingerprint), r.Fingerprint)
	}
}

// TestReadingWithLargeFingerPrintSizeAndFileLargerThanScannerBuf tests for reading of log file when:
// - fingerprint size is larger than the size of scanner default buffer (defaultBufSize)
// - size of the log file is lower than fingerprint size
func TestReadingWithLargeFingerPrintSizeAndFileLargerThanScannerBuf(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Generate log lines
	body := "abcdefghijklmnopqrstuvwxyz1234567890"
	fileContent := ""
	expected := [][]byte{}
	fingerPrintSize := scanner.DefaultBufferSize + 2*1024

	for i := 0; len(fileContent) < fingerPrintSize-1024; i++ {
		log := fmt.Sprintf("line %d log %s, end of line %d", i, body, i)
		fileContent += fmt.Sprintf("%s\n", log)
		expected = append(expected, []byte(log))
	}

	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, fileContent)

	f, sink := testFactory(t,
		withFingerprintSize(fingerPrintSize),
		withMaxLogSize(defaultMaxLogSize),
		withSinkChanSize(1000),
	)

	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	initialFingerPrintSize := r.Fingerprint.Len()
	r.ReadToEnd(context.Background())
	require.Equal(t, initialFingerPrintSize, r.Fingerprint.Len())

	sink.ExpectTokens(t, expected...)
}
