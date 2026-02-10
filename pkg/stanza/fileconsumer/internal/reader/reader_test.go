// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"context"
	"fmt"
	"os"
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
