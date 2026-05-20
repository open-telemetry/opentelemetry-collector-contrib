// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"golang.org/x/text/encoding/unicode"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/scanner"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/stanzatime"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

func TestFileReader_FingerprintUpdated(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	tempCopy := filetest.OpenFile(t, temp.Name())

	f, sink := testFactory(t)
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)

	reader, err := f.NewReader(tempCopy, fp)
	require.NoError(t, err)
	defer reader.Close()

	filetest.WriteString(t, temp, "testlog1\n")
	reader.ReadToEnd(t.Context())
	sink.ExpectToken(t, []byte("testlog1"))
	require.Equal(t, fingerprint.New([]byte("testlog1\n")), reader.Fingerprint)
}

// Test that a fingerprint:
// - Starts empty
// - Updates as a file is read
// - Stops updating when the max fingerprint size is reached
// - Stops exactly at max fingerprint size, regardless of content
func TestFingerprintGrowsAndStops(t *testing.T) {
	t.Parallel()

	// Use a number with many factors.
	// Sometimes fingerprint length will align with
	// the end of a line, sometimes not. Test both.
	fpSize := 360

	// Use prime numbers to ensure variation in
	// whether or not they are factors of fpSize
	lineLens := []int{3, 5, 7, 11, 13, 17, 19, 23, 27}

	for _, lineLen := range lineLens {
		t.Run(fmt.Sprintf("%d", lineLen), func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			temp := filetest.OpenTemp(t, tempDir)
			tempCopy := filetest.OpenFile(t, temp.Name())

			f, _ := testFactory(t, withSinkChanSize(3*fpSize/lineLen), withFingerprintSize(fpSize))
			fp, err := f.NewFingerprint(temp)
			require.NoError(t, err)
			require.Equal(t, fingerprint.New([]byte("")), fp)

			reader, err := f.NewReader(tempCopy, fp)
			require.NoError(t, err)
			defer reader.Close()

			// keep track of what has been written to the file
			var fileContent []byte

			// keep track of expected fingerprint size
			expectedFP := 0

			// Write lines until file is much larger than the length of the fingerprint
			for len(fileContent) < 2*fpSize {
				expectedFP += lineLen
				if expectedFP > fpSize {
					expectedFP = fpSize
				}

				line := string(filetest.TokenWithLength(lineLen-1)) + "\n"
				fileContent = append(fileContent, []byte(line)...)

				filetest.WriteString(t, temp, line)
				reader.ReadToEnd(t.Context())
				require.Equal(t, fingerprint.New(fileContent[:expectedFP]), reader.Fingerprint)
			}
		})
	}
}

// This is same test like TestFingerprintGrowsAndStops, but with additional check for fingerprint size check
// Test that a fingerprint:
// - Starts empty
// - Updates as a file is read
// - Stops updating when the max fingerprint size is reached
// - Stops exactly at max fingerprint size, regardless of content
// - Do not change size after fingerprint configuration change
func TestFingerprintChangeSize(t *testing.T) {
	t.Parallel()

	// Use a number with many factors.
	// Sometimes fingerprint length will align with
	// the end of a line, sometimes not. Test both.
	fpSize := 360

	// Use prime numbers to ensure variation in
	// whether or not they are factors of fpSize
	lineLens := []int{3, 4, 5, 6, 7, 8, 11, 12, 13, 17, 19, 23, 27, 36}

	for _, lineLen := range lineLens {
		t.Run(fmt.Sprintf("%d", lineLen), func(t *testing.T) {
			t.Parallel()

			f, _ := testFactory(t, withSinkChanSize(3*fpSize/lineLen), withFingerprintSize(fpSize))

			tempDir := t.TempDir()
			temp := filetest.OpenTemp(t, tempDir)

			fp, err := f.NewFingerprint(temp)
			require.NoError(t, err)
			require.Equal(t, fingerprint.New([]byte("")), fp)

			reader, err := f.NewReader(filetest.OpenFile(t, temp.Name()), fp)
			require.NoError(t, err)

			// keep track of what has been written to the file
			var fileContent []byte

			// keep track of expected fingerprint size
			expectedFP := 0

			// Write lines until file is much larger than the length of the fingerprint
			for len(fileContent) < 2*fpSize {
				expectedFP += lineLen
				if expectedFP > fpSize {
					expectedFP = fpSize
				}

				line := string(filetest.TokenWithLength(lineLen-1)) + "\n"
				fileContent = append(fileContent, []byte(line)...)

				filetest.WriteString(t, temp, line)
				reader.ReadToEnd(t.Context())
				require.Equal(t, fingerprint.New(fileContent[:expectedFP]), reader.Fingerprint)
			}

			// Recreate the factory with a larger fingerprint size
			fpSizeUp := fpSize * 2
			f, _ = testFactory(t, withSinkChanSize(3*fpSize/lineLen), withFingerprintSize(fpSizeUp))

			// Recreate the reader with the new factory
			reader, err = f.NewReaderFromMetadata(filetest.OpenFile(t, temp.Name()), reader.Close())
			require.NoError(t, err)

			line := string(filetest.TokenWithLength(lineLen-1)) + "\n"
			fileContent = append(fileContent, []byte(line)...)

			filetest.WriteString(t, temp, line)
			reader.ReadToEnd(t.Context())
			require.Equal(t, fingerprint.New(fileContent[:fpSizeUp]), reader.Fingerprint)

			// Recreate the factory with a smaller fingerprint size
			fpSizeDown := fpSize / 2
			f, _ = testFactory(t, withSinkChanSize(3*fpSize/lineLen), withFingerprintSize(fpSizeDown))

			// Recreate the reader with the new factory
			reader, err = f.NewReaderFromMetadata(filetest.OpenFile(t, temp.Name()), reader.Close())
			require.NoError(t, err)

			line = string(filetest.TokenWithLength(lineLen-1)) + "\n"
			fileContent = append(fileContent, []byte(line)...)

			filetest.WriteString(t, temp, line)
			reader.ReadToEnd(t.Context())
			require.Equal(t, fingerprint.New(fileContent[:fpSizeDown]), reader.Fingerprint)
		})
	}
}

func TestFlushPeriodEOF(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)

	// Create a long enough initial token, so the scanner can't read the whole file at once
	aContentLength := 2 * 16 * 1024
	content := []byte(strings.Repeat("a", aContentLength))
	content = append(content, '\n', 'b')
	_, err := temp.WriteString(string(content))
	require.NoError(t, err)

	flushPeriod := time.Millisecond
	f, sink := testFactory(t, withFlushPeriod(flushPeriod))
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.Offset)

	clock := stanzatime.NewAlwaysIncreasingClock()
	stanzatime.Now = clock.Now
	stanzatime.Since = clock.Since
	defer func() {
		stanzatime.Now = time.Now
		stanzatime.Since = time.Since
	}()

	// First ReadToEnd should not emit only the terminated token
	r.ReadToEnd(t.Context())
	sink.ExpectToken(t, content[0:aContentLength])

	// Advance time past the flush period
	clock.Advance(2 * flushPeriod)

	// Second ReadToEnd should emit the unterminated token because of flush timeout
	r.ReadToEnd(t.Context())
	sink.ExpectToken(t, []byte{'b'})
}

func TestUntermintedLongLogEntry(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)

	// Create a log entry longer than DefaultBufferSize (16KB) but shorter than maxLogSize
	content := filetest.TokenWithLength(20 * 1024) // 20KB
	_, err := temp.WriteString(string(content))    // no newline
	require.NoError(t, err)

	// Use a controlled clock. It advances by 1ns each time Now() is called, which may happen
	// a few times during a call to ReadToEnd.
	clock := stanzatime.NewAlwaysIncreasingClock()
	stanzatime.Now = clock.Now
	stanzatime.Since = clock.Since
	defer func() {
		stanzatime.Now = time.Now
		stanzatime.Since = time.Since
	}()

	// Use a long flush period to ensure it does not expire DURING a ReadToEnd
	flushPeriod := time.Second

	f, sink := testFactory(t, withFlushPeriod(flushPeriod))
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.Offset)

	// First ReadToEnd should not emit anything as flush period hasn't expired
	r.ReadToEnd(t.Context())
	sink.ExpectNoCalls(t)

	// Advance time past the flush period to test behavior after timer is expired
	clock.Advance(2 * flushPeriod)

	// Second ReadToEnd should emit the full untruncated token
	r.ReadToEnd(t.Context())
	sink.ExpectToken(t, content)

	sink.ExpectNoCalls(t)
}

func TestUntermintedLogEntryGrows(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)

	// Create a log entry longer than DefaultBufferSize (16KB) but shorter than maxLogSize
	content := filetest.TokenWithLength(20 * 1024) // 20KB
	_, err := temp.WriteString(string(content))    // no newline
	require.NoError(t, err)

	// Use a controlled clock. It advances by 1ns each time Now() is called, which may happen
	// a few times during a call to ReadToEnd.
	clock := stanzatime.NewAlwaysIncreasingClock()
	stanzatime.Now = clock.Now
	stanzatime.Since = clock.Since
	defer func() {
		stanzatime.Now = time.Now
		stanzatime.Since = time.Since
	}()

	// Use a long flush period to ensure it does not expire DURING a ReadToEnd
	flushPeriod := time.Second

	f, sink := testFactory(t, withFlushPeriod(flushPeriod))
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.Offset)

	// First ReadToEnd should not emit anything as flush period hasn't expired
	r.ReadToEnd(t.Context())
	sink.ExpectNoCalls(t)

	// Advance time past the flush period to test behavior after timer is expired
	clock.Advance(2 * flushPeriod)

	// Write additional unterminated content to ensure all is picked up in the same token
	// The flusher should notice new data and not return anything on the next call
	additionalContext := filetest.TokenWithLength(1024)
	_, err = temp.WriteString(string(additionalContext)) // no newline
	require.NoError(t, err)

	r.ReadToEnd(t.Context())
	sink.ExpectNoCalls(t)

	// Advance time past the flush period to test behavior after timer is expired
	clock.Advance(2 * flushPeriod)

	// Finally, since we haven't seen new data, flusher should emit the token
	r.ReadToEnd(t.Context())
	sink.ExpectToken(t, append(content, additionalContext...))

	sink.ExpectNoCalls(t)
}

// TestReadToEnd_OffsetPreservedOnEmitError is a regression test for
// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/47908.
//
// emit.Callback explicitly permits an error return, but readContents previously
// advanced r.Offset unconditionally after every emitFunc call. Tokens whose
// batch was rejected were skipped on the next poll, producing silent data loss.
//
// readContents must instead leave r.Offset untouched whenever emitFunc returns
// a non-nil error, so the same bytes are re-read (and re-attempted) on the next
// ReadToEnd cycle.
func TestReadToEnd_OffsetPreservedOnEmitError(t *testing.T) {
	errEmit := errors.New("emit failed")

	t.Run("eof_path_single_batch_fails", func(t *testing.T) {
		// Fewer lines than DefaultMaxBatchSize: the only emit happens on the EOF path.
		tempDir := t.TempDir()
		temp := filetest.OpenTemp(t, tempDir)
		const content = "line1\nline2\nline3\n"
		filetest.WriteString(t, temp, content)

		var calls atomic.Int64
		f := newTestFactory(t, func(context.Context, [][]byte, map[string]any, int64, []int64) error {
			calls.Add(1)
			return errEmit
		})
		fp, err := f.NewFingerprint(temp)
		require.NoError(t, err)
		r, err := f.NewReader(filetest.OpenFile(t, temp.Name()), fp)
		require.NoError(t, err)
		defer r.Close()

		r.ReadToEnd(t.Context())

		assert.Equal(t, int64(1), calls.Load(), "emit should run exactly once at EOF")
		assert.Equal(t, int64(0), r.Offset,
			"offset must not advance when EOF emit fails — those tokens are still owed downstream")
	})

	t.Run("batch_full_path_first_batch_fails", func(t *testing.T) {
		// More lines than DefaultMaxBatchSize so a mid-loop emit fires before EOF.
		// On emit error readContents must bail out — otherwise the offset moves to
		// EOF and subsequent batches are skipped silently.
		const totalLines = DefaultMaxBatchSize * 2
		tempDir := t.TempDir()
		temp := filetest.OpenTemp(t, tempDir)
		var buf strings.Builder
		for i := range totalLines {
			fmt.Fprintf(&buf, "line%04d\n", i) // 9 bytes/line
		}
		filetest.WriteString(t, temp, buf.String())

		var calls atomic.Int64
		f := newTestFactory(t, func(context.Context, [][]byte, map[string]any, int64, []int64) error {
			calls.Add(1)
			return errEmit
		})
		fp, err := f.NewFingerprint(temp)
		require.NoError(t, err)
		r, err := f.NewReader(filetest.OpenFile(t, temp.Name()), fp)
		require.NoError(t, err)
		defer r.Close()

		r.ReadToEnd(t.Context())

		assert.Equal(t, int64(1), calls.Load(),
			"reader must stop after the first failing batch instead of plowing through %d batches",
			totalLines/DefaultMaxBatchSize)
		assert.Equal(t, int64(0), r.Offset,
			"offset must not advance when the first mid-loop emit fails")
	})

	t.Run("eof_after_successful_batch_fails", func(t *testing.T) {
		// First batch (DefaultMaxBatchSize lines) succeeds; trailing EOF emit fails.
		// Offset must reflect the first successful batch only, not the trailing tail.
		const successfulLines = DefaultMaxBatchSize
		const trailingLines = DefaultMaxBatchSize / 2
		const lineWidth = int64(9) // "lineNNNN\n"
		tempDir := t.TempDir()
		temp := filetest.OpenTemp(t, tempDir)
		var buf strings.Builder
		for i := range successfulLines + trailingLines {
			fmt.Fprintf(&buf, "line%04d\n", i)
		}
		filetest.WriteString(t, temp, buf.String())

		var calls atomic.Int64
		f := newTestFactory(t, func(context.Context, [][]byte, map[string]any, int64, []int64) error {
			if calls.Add(1) == 1 {
				return nil
			}
			return errEmit
		})
		fp, err := f.NewFingerprint(temp)
		require.NoError(t, err)
		r, err := f.NewReader(filetest.OpenFile(t, temp.Name()), fp)
		require.NoError(t, err)
		defer r.Close()

		r.ReadToEnd(t.Context())

		assert.Equal(t, int64(2), calls.Load(),
			"both the batch-full emit and the trailing EOF emit should run")
		assert.Equal(t, int64(successfulLines)*lineWidth, r.Offset,
			"offset must stop at the end of the last successful batch, not at EOF")
	})
}

// TestReadToEnd_RecordNumRolledBackOnEmitError pins that when emitFunc rejects
// a batch, RecordNum is rewound by the size of that batch so the next poll
// re-reads the same bytes and assigns the same record_number values to those
// records (instead of skipping the failed batch's IDs and producing a gap).
func TestReadToEnd_RecordNumRolledBackOnEmitError(t *testing.T) {
	errEmit := errors.New("emit failed")

	t.Run("eof_path", func(t *testing.T) {
		tempDir := t.TempDir()
		temp := filetest.OpenTemp(t, tempDir)
		filetest.WriteString(t, temp, "line1\nline2\nline3\n")

		f := newTestFactory(t, func(context.Context, [][]byte, map[string]any, int64, []int64) error {
			return errEmit
		})
		fp, err := f.NewFingerprint(temp)
		require.NoError(t, err)
		r, err := f.NewReader(filetest.OpenFile(t, temp.Name()), fp)
		require.NoError(t, err)
		defer r.Close()

		r.ReadToEnd(t.Context())
		assert.Equal(t, int64(0), r.RecordNum,
			"RecordNum must be rewound when an EOF emit fails so the retry assigns the same values")
	})

	t.Run("batch_full_path", func(t *testing.T) {
		const totalLines = DefaultMaxBatchSize * 2
		tempDir := t.TempDir()
		temp := filetest.OpenTemp(t, tempDir)
		var buf strings.Builder
		for i := range totalLines {
			fmt.Fprintf(&buf, "line%04d\n", i)
		}
		filetest.WriteString(t, temp, buf.String())

		f := newTestFactory(t, func(context.Context, [][]byte, map[string]any, int64, []int64) error {
			return errEmit
		})
		fp, err := f.NewFingerprint(temp)
		require.NoError(t, err)
		r, err := f.NewReader(filetest.OpenFile(t, temp.Name()), fp)
		require.NoError(t, err)
		defer r.Close()

		r.ReadToEnd(t.Context())
		assert.Equal(t, int64(0), r.RecordNum,
			"RecordNum must be rewound when a mid-loop emit fails")
	})
}

// TestReadToEnd_DeleteAtEOFSkippedOnEmitError pins that delete_after_read does
// not delete the source file when the trailing emit fails. The retried poll
// needs the file on disk; deleting at EOF before emit succeeds turns an emit
// error into permanent data loss.
func TestReadToEnd_DeleteAtEOFSkippedOnEmitError(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	filetest.WriteString(t, temp, "line1\nline2\nline3\n")

	f := newTestFactory(t, func(context.Context, [][]byte, map[string]any, int64, []int64) error {
		return errors.New("emit failed")
	})
	f.DeleteAtEOF = true
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(filetest.OpenFile(t, temp.Name()), fp)
	require.NoError(t, err)
	defer r.Close()

	r.ReadToEnd(t.Context())

	_, statErr := os.Stat(temp.Name())
	assert.NoError(t, statErr,
		"file must still exist after emit error so the next poll can re-read it")
}

// TestReadToEnd_GzipOffsetPreservedOnEmitError pins that for gzip-compressed
// files the deferred `Offset = currentEOF` jump in ReadToEnd is skipped when
// emitFunc returns an error. Without this gating the compressed-file offset
// would advance to EOF and the rejected batch would be silently lost on the
// next poll (the original concern paulojmdias raised in review).
func TestReadToEnd_GzipOffsetPreservedOnEmitError(t *testing.T) {
	tempDir := t.TempDir()
	gzipPath := filepath.Join(tempDir, "test.log.gz")
	gzipFile, err := os.Create(gzipPath)
	require.NoError(t, err)
	gw := gzip.NewWriter(gzipFile)
	_, err = gw.Write([]byte("line1\nline2\nline3\n"))
	require.NoError(t, err)
	require.NoError(t, gw.Close())
	require.NoError(t, gzipFile.Close())

	f := newTestFactory(t, func(context.Context, [][]byte, map[string]any, int64, []int64) error {
		return errors.New("emit failed")
	})
	f.Compression = "gzip"
	opened := filetest.OpenFile(t, gzipPath)
	fp, err := f.NewFingerprint(opened)
	require.NoError(t, err)
	r, err := f.NewReader(opened, fp)
	require.NoError(t, err)
	defer r.Close()

	r.ReadToEnd(t.Context())

	assert.Equal(t, int64(0), r.Offset,
		"gzip Offset must not jump to currentEOF when the emit failed; it would silently skip the rejected batch")
}

func BenchmarkFileRead(b *testing.B) {
	tempDir := b.TempDir()

	temp := filetest.OpenTemp(b, tempDir)
	// Initialize the file to ensure a unique fingerprint
	_, err := temp.WriteString(temp.Name() + "\n")
	require.NoError(b, err)
	// Write half the content before starting the benchmark
	for range 100 {
		_, err := temp.WriteString(string(filetest.TokenWithLength(999)) + "\n")
		require.NoError(b, err)
	}

	// Use a long flush period to ensure it does not expire DURING a ReadToEnd
	counter := atomic.Int64{}
	f := newTestFactory(b, func(_ context.Context, tokens [][]byte, _ map[string]any, _ int64, _ []int64) error {
		counter.Add(int64(len(tokens)))
		return nil
	})
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		file, err := os.OpenFile(temp.Name(), os.O_CREATE|os.O_RDWR, 0o600)
		require.NoError(b, err)
		fp, err := f.NewFingerprint(file)
		require.NoError(b, err)
		reader, err := f.NewReader(file, fp)
		require.NoError(b, err)
		reader.ReadToEnd(b.Context())
		assert.EqualValues(b, (i+1)*101, counter.Load())
		reader.Close()
	}
}

func newTestFactory(tb testing.TB, callback emit.Callback) *Factory {
	splitFunc, err := split.Config{}.Func(unicode.UTF8, false, defaultMaxLogSize)
	require.NoError(tb, err)

	return &Factory{
		TelemetrySettings: componenttest.NewNopTelemetrySettings(),
		FromBeginning:     true,
		FingerprintSize:   fingerprint.DefaultSize,
		InitialBufferSize: scanner.DefaultBufferSize,
		MaxLogSize:        defaultMaxLogSize,
		Encoding:          unicode.UTF8,
		SplitFunc:         splitFunc,
		TrimFunc:          trim.Whitespace,
		FlushTimeout:      defaultFlushPeriod,
		EmitFunc:          callback,
		Attributes: attrs.Resolver{
			IncludeFileName: true,
		},
	}
}
