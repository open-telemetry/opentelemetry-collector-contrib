// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/regex"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
)

func TestPersistFlusher(t *testing.T) {
	flushPeriod := 100 * time.Millisecond
	f, sink := testFactory(t, withFlushPeriod(flushPeriod))

	temp := filetest.OpenTemp(t, t.TempDir())
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	_, err = temp.WriteString("log with newline\nlog without newline")
	require.NoError(t, err)

	// ReadToEnd will return when we hit eof, but we shouldn't emit the unfinished log yet
	r.ReadToEnd(context.Background())
	sink.ExpectToken(t, []byte("log with newline"))

	// Even trying again shouldn't produce the log yet because the flush period still hasn't expired.
	r.ReadToEnd(context.Background())
	sink.ExpectNoCallsUntil(t, 2*flushPeriod)

	// A copy of the reader should remember that we last emitted about 200ms ago.
	copyReader, err := f.NewReaderFromMetadata(temp, r.Metadata)
	assert.NoError(t, err)

	// This time, the flusher will kick in and we should emit the unfinished log.
	// If the copy did not remember when we last emitted a log, then the flushPeriod
	// will not be expired at this point so we won't see the unfinished log.
	copyReader.ReadToEnd(context.Background())
	sink.ExpectToken(t, []byte("log without newline"))
}

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
			f, sink := testFactory(t)

			temp := filetest.OpenTemp(t, t.TempDir())
			_, err := temp.Write(tc.fileContent)
			require.NoError(t, err)

			fp, err := f.NewFingerprint(temp)
			require.NoError(t, err)

			r, err := f.NewReader(temp, fp)
			require.NoError(t, err)

			r.ReadToEnd(context.Background())

			for _, expected := range tc.expected {
				require.Equal(t, expected, sink.NextToken(t))
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

	f, sink := testFactory(t, withMaxLogSize(10))

	temp := filetest.OpenTemp(t, t.TempDir())
	_, err := temp.Write(fileContent)
	require.NoError(t, err)

	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	r.ReadToEnd(context.Background())

	for _, expected := range expected {
		require.Equal(t, expected, sink.NextToken(t))
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

	sCfg := split.Config{LineStartPattern: `\d+-\d+-\d+`}
	f, sink := testFactory(t, withSplitConfig(sCfg), withMaxLogSize(15))

	temp := filetest.OpenTemp(t, t.TempDir())
	_, err := temp.Write(fileContent)
	require.NoError(t, err)

	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	r.ReadToEnd(context.Background())

	for _, expected := range expected {
		require.Equal(t, expected, sink.NextToken(t))
	}
}

func TestHeaderFingerprintIncluded(t *testing.T) {
	fileContent := []byte("#header-line\naaa\n")

	f, _ := testFactory(t, withMaxLogSize(10))

	regexConf := regex.NewConfig()
	regexConf.Regex = "^#(?P<header>.*)"

	enc, err := decode.LookupEncoding("utf-8")
	require.NoError(t, err)

	h, err := header.NewConfig("^#", []operator.Config{{Builder: regexConf}}, enc)
	require.NoError(t, err)
	f.HeaderConfig = h

	temp := filetest.OpenTemp(t, t.TempDir())

	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	_, err = temp.Write(fileContent)
	require.NoError(t, err)

	r.ReadToEnd(context.Background())

	require.Equal(t, []byte("#header-line\naaa\n"), r.Fingerprint.FirstBytes)
}
