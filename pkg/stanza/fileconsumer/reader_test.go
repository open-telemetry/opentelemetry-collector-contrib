// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

func TestTokenization(t *testing.T) {
	testCases := []struct {
		testName    string
		fileContent []byte
		expected    [][]byte
	}{
		{
			"simple",
			[]byte("testlog1\ntestlog2\n"),
			[][]byte{
				[]byte("testlog1"),
				[]byte("testlog2"),
			},
		},
		{
			"empty_only",
			[]byte("\n"),
			[][]byte{
				[]byte(""),
			},
		},
		{
			"empty_first",
			[]byte("\ntestlog1\ntestlog2\n"),
			[][]byte{
				[]byte(""),
				[]byte("testlog1"),
				[]byte("testlog2"),
			},
		},
		{
			"empty_between_lines",
			[]byte("testlog1\n\ntestlog2\n"),
			[][]byte{
				[]byte("testlog1"),
				[]byte(""),
				[]byte("testlog2"),
			},
		},
		{
			"multiple_empty",
			[]byte("\n\ntestlog1\n\n\ntestlog2\n"),
			[][]byte{
				[]byte(""),
				[]byte(""),
				[]byte("testlog1"),
				[]byte(""),
				[]byte(""),
				[]byte("testlog2"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			f, emitChan := testReaderFactory(t)

			temp := openTemp(t, t.TempDir())
			_, err := temp.Write(tc.fileContent)
			require.NoError(t, err)

			r, err := f.newReaderBuilder().withFile(temp).build()
			require.NoError(t, err)

			r.ReadToEnd(context.Background())

			for _, expected := range tc.expected {
				require.Equal(t, expected, readToken(t, emitChan))
			}
		})
	}
}

func TestTokenizationTooLong(t *testing.T) {
	fileContent := []byte("aaaaaaaaaaaaaaaaaaaaaa\naaa\n")
	expected := [][]byte{
		[]byte("aaaaaaaaaa"),
		[]byte("aaaaaaaaaa"),
		[]byte("aa"),
		[]byte("aaa"),
	}

	f, emitChan := testReaderFactory(t)
	f.readerConfig.maxLogSize = 10

	temp := openTemp(t, t.TempDir())
	_, err := temp.Write(fileContent)
	require.NoError(t, err)

	r, err := f.newReaderBuilder().withFile(temp).build()
	require.NoError(t, err)

	r.ReadToEnd(context.Background())

	for _, expected := range expected {
		require.Equal(t, expected, readToken(t, emitChan))
	}
}

func TestTokenizationTooLongWithLineStartPattern(t *testing.T) {
	fileContent := []byte("aaa2023-01-01aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa 2023-01-01 2 2023-01-01")
	expected := [][]byte{
		[]byte("aaa"),
		[]byte("2023-01-01aaaaa"),
		[]byte("aaaaaaaaaaaaaaa"),
		[]byte("aaaaaaaaaaaaaaa"),
		[]byte("aaaaa"),
		[]byte("2023-01-01 2"),
	}

	f, emitChan := testReaderFactory(t)

	mlc := helper.NewMultilineConfig()
	mlc.LineStartPattern = `\d+-\d+-\d+`
	f.splitterFactory = newMultilineSplitterFactory(helper.SplitterConfig{
		EncodingConfig: helper.NewEncodingConfig(),
		Flusher:        helper.NewFlusherConfig(),
		Multiline:      mlc,
	})
	f.readerConfig.maxLogSize = 15

	temp := openTemp(t, t.TempDir())
	_, err := temp.Write(fileContent)
	require.NoError(t, err)

	r, err := f.newReaderBuilder().withFile(temp).build()
	require.NoError(t, err)

	r.ReadToEnd(context.Background())
	require.True(t, r.eof)

	for _, expected := range expected {
		require.Equal(t, expected, readToken(t, emitChan))
	}
}

func TestHeaderFingerprintIncluded(t *testing.T) {
	fileContent := []byte("#header-line\naaa\n")

	f, _ := testReaderFactory(t)
	f.readerConfig.maxLogSize = 10

	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header>.*)"

	headerConf := &HeaderConfig{
		Pattern: "^#",
		MetadataOperators: []operator.Config{
			{
				Builder: regexConf,
			},
		},
	}

	enc, err := helper.EncodingConfig{
		Encoding: "utf-8",
	}.Build()
	require.NoError(t, err)

	h, err := headerConf.buildHeaderSettings(enc.Encoding)
	require.NoError(t, err)
	f.headerSettings = h

	temp := openTemp(t, t.TempDir())

	r, err := f.newReaderBuilder().withFile(temp).build()
	require.NoError(t, err)

	_, err = temp.Write(fileContent)
	require.NoError(t, err)

	r.ReadToEnd(context.Background())

	require.Equal(t, fileContent, r.Fingerprint.FirstBytes)
}

func testReaderFactory(t *testing.T) (*readerFactory, chan *emitParams) {
	emitChan := make(chan *emitParams, 100)
	splitterConfig := helper.NewSplitterConfig()
	return &readerFactory{
		SugaredLogger: testutil.Logger(t),
		readerConfig: &readerConfig{
			fingerprintSize: DefaultFingerprintSize,
			maxLogSize:      defaultMaxLogSize,
			emit:            testEmitFunc(emitChan),
		},
		fromBeginning:   true,
		splitterFactory: newMultilineSplitterFactory(splitterConfig),
		encodingConfig:  splitterConfig.EncodingConfig,
	}, emitChan
}

func readToken(t *testing.T, c chan *emitParams) []byte {
	select {
	case call := <-c:
		return call.token
	case <-time.After(3 * time.Second):
		require.FailNow(t, "Timed out waiting for token")
	}
	return nil
}

func TestMapCopy(t *testing.T) {
	initMap := map[string]any{
		"mapVal": map[string]any{
			"nestedVal": "value1",
		},
		"intVal": 1,
		"strVal": "OrigStr",
	}

	copyMap := mapCopy(initMap)
	// Mutate values on the copied map
	copyMap["mapVal"].(map[string]any)["nestedVal"] = "overwrittenValue"
	copyMap["intVal"] = 2
	copyMap["strVal"] = "CopyString"

	// Assert that the original map should have the same values
	assert.Equal(t, "value1", initMap["mapVal"].(map[string]any)["nestedVal"])
	assert.Equal(t, 1, initMap["intVal"])
	assert.Equal(t, "OrigStr", initMap["strVal"])
}

func TestEncodingDecode(t *testing.T) {
	testFile := openTemp(t, t.TempDir())
	testToken := tokenWithLength(2 * DefaultFingerprintSize)
	_, err := testFile.Write(testToken)
	require.NoError(t, err)
	fp, err := NewFingerprint(testFile, DefaultFingerprintSize)
	require.NoError(t, err)

	f := readerFactory{
		SugaredLogger: testutil.Logger(t),
		readerConfig: &readerConfig{
			fingerprintSize: DefaultFingerprintSize,
			maxLogSize:      defaultMaxLogSize,
		},
		splitterFactory: newMultilineSplitterFactory(helper.NewSplitterConfig()),
		fromBeginning:   false,
	}
	r, err := f.newReader(testFile, fp)
	require.NoError(t, err)

	// Just faking out these properties
	r.HeaderFinalized = true
	r.FileAttributes.HeaderAttributes = map[string]any{"foo": "bar"}

	assert.Equal(t, testToken[:DefaultFingerprintSize], r.Fingerprint.FirstBytes)
	assert.Equal(t, int64(2*DefaultFingerprintSize), r.Offset)

	// Encode
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	require.NoError(t, enc.Encode(r))

	// Decode
	dec := json.NewDecoder(bytes.NewReader(buf.Bytes()))
	decodedReader, err := f.unsafeReader()
	require.NoError(t, err)
	require.NoError(t, dec.Decode(decodedReader))

	// Assert decoded reader has values persisted
	assert.Equal(t, testToken[:DefaultFingerprintSize], decodedReader.Fingerprint.FirstBytes)
	assert.Equal(t, int64(2*DefaultFingerprintSize), decodedReader.Offset)
	assert.True(t, decodedReader.HeaderFinalized)
	assert.Equal(t, map[string]any{"foo": "bar"}, decodedReader.FileAttributes.HeaderAttributes)

	// These fields are intentionally excluded, as they may have changed
	assert.Empty(t, decodedReader.FileAttributes.Name)
	assert.Empty(t, decodedReader.FileAttributes.Path)
	assert.Empty(t, decodedReader.FileAttributes.NameResolved)
	assert.Empty(t, decodedReader.FileAttributes.PathResolved)
}

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
			name:              "tiny_max_log_size_from_start",
			fingerprintSize:   10,
			maxLogSize:        2,
			fromBeginning:     true,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("12345\n67890"),
			expectTokens:      [][]byte{[]byte("fo"), []byte("o1"), []byte("23"), []byte("45"), []byte("67"), []byte("89")},
			expectOffset:      13,
			expectFingerprint: []byte("foo12345\n6"),
		},
		{
			name:              "tiny_max_log_size",
			fingerprintSize:   10,
			maxLogSize:        2,
			initBytes:         []byte("foo"),
			moreBytes:         []byte("12345\n67890"),
			expectTokens:      [][]byte{[]byte("12"), []byte("34"), []byte("5"), []byte("67"), []byte("89")},
			expectOffset:      13,
			expectFingerprint: []byte("foo12345\n6"),
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
		splitterConfig := helper.NewSplitterConfig()
		emitChan := make(chan *emitParams, 100)
		f := &readerFactory{
			SugaredLogger: testutil.Logger(t),
			readerConfig: &readerConfig{
				fingerprintSize: tc.fingerprintSize,
				maxLogSize:      tc.maxLogSize,
				bufferSize:      bufferSize,
				emit:            testEmitFunc(emitChan),
			},
			fromBeginning:   tc.fromBeginning,
			splitterFactory: newMultilineSplitterFactory(splitterConfig),
			encodingConfig:  splitterConfig.EncodingConfig,
		}

		temp := openTemp(t, t.TempDir())
		_, err := temp.Write(tc.initBytes)
		require.NoError(t, err)

		fi, err := temp.Stat()
		require.NoError(t, err)
		require.Equal(t, int64(len(tc.initBytes)), fi.Size())

		fp, err := NewFingerprint(temp, tc.fingerprintSize)
		require.NoError(t, err)
		r, err := f.newReader(temp, fp)
		require.NoError(t, err)
		require.Same(t, temp, r.file)

		if tc.fromBeginning {
			assert.Equal(t, int64(0), r.Offset)
		} else {
			assert.Equal(t, int64(len(tc.initBytes)), r.Offset)
		}
		assert.Equal(t, tc.initBytes, r.Fingerprint.FirstBytes)

		i, err := temp.Write(tc.moreBytes)
		require.NoError(t, err)
		require.Equal(t, i, len(tc.moreBytes))

		r.ReadToEnd(context.Background())

		for _, token := range tc.expectTokens {
			tk := readToken(t, emitChan)
			require.Equal(t, token, tk)
		}
		assert.Equal(t, tc.expectOffset, r.Offset)
		assert.Equal(t, tc.expectFingerprint, r.Fingerprint.FirstBytes)
	}
}
