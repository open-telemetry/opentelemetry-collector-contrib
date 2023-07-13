// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
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

	require.Equal(t, []byte("#header-line\naaa\n"), r.Fingerprint.FirstBytes)
}

func testReaderFactory(t *testing.T) (*readerFactory, chan *emitParams) {
	emitChan := make(chan *emitParams, 100)
	splitterConfig := helper.NewSplitterConfig()
	return &readerFactory{
		SugaredLogger: testutil.Logger(t),
		readerConfig: &readerConfig{
			fingerprintSize: fingerprint.DefaultSize,
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

func TestEncodingDecode(t *testing.T) {
	testFile := openTemp(t, t.TempDir())
	testToken := tokenWithLength(2 * fingerprint.DefaultSize)
	_, err := testFile.Write(testToken)
	require.NoError(t, err)
	fp, err := fingerprint.New(testFile, fingerprint.DefaultSize)
	require.NoError(t, err)

	f := readerFactory{
		SugaredLogger: testutil.Logger(t),
		readerConfig: &readerConfig{
			fingerprintSize: fingerprint.DefaultSize,
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

	assert.Equal(t, testToken[:fingerprint.DefaultSize], r.Fingerprint.FirstBytes)
	assert.Equal(t, int64(2*fingerprint.DefaultSize), r.Offset)

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
	assert.Equal(t, testToken[:fingerprint.DefaultSize], decodedReader.Fingerprint.FirstBytes)
	assert.Equal(t, int64(2*fingerprint.DefaultSize), decodedReader.Offset)
	assert.True(t, decodedReader.HeaderFinalized)
	assert.Equal(t, map[string]any{"foo": "bar"}, decodedReader.FileAttributes.HeaderAttributes)

	// These fields are intentionally excluded, as they may have changed
	assert.Empty(t, decodedReader.FileAttributes.Name)
	assert.Empty(t, decodedReader.FileAttributes.Path)
	assert.Empty(t, decodedReader.FileAttributes.NameResolved)
	assert.Empty(t, decodedReader.FileAttributes.PathResolved)
}
