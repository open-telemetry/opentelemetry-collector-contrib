// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/fingerprint"
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
	reader.ReadToEnd(context.Background())
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
		lineLen := lineLen
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
				reader.ReadToEnd(context.Background())
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
		lineLen := lineLen
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
				reader.ReadToEnd(context.Background())
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
			reader.ReadToEnd(context.Background())
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
			reader.ReadToEnd(context.Background())
			require.Equal(t, fingerprint.New(fileContent[:fpSizeDown]), reader.Fingerprint)
		})
	}
}

func TestFlushPeriodEOF(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test on Windows; See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/32715")
	}
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	// Create a long enough initial token, so the scanner can't read the whole file at once
	aContentLength := 2 * 16 * 1024
	content := []byte(strings.Repeat("a", aContentLength))
	content = append(content, '\n', 'b')
	_, err := temp.WriteString(string(content))
	require.NoError(t, err)

	// Make sure FlushPeriod is small, so it is guaranteed to expire
	f, sink := testFactory(t, withFlushPeriod(5*time.Nanosecond))
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.Offset)

	r.ReadToEnd(context.Background())
	sink.ExpectTokens(t, content[0:aContentLength], []byte{'b'})
}

func TestFileReaderMaxSurgeSizeInitialFileTooLarge(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	// Create a token bigger than the max surge size
	aContentLength := 2 * 1024 // More than 1Kb
	content := []byte(strings.Repeat("a", aContentLength))
	_, err := temp.WriteString(string(content))
	require.NoError(t, err)

	f, sink := testFactory(t, withMaxSurgeSize(1*1024)) // 1Kb
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)
	assert.Equal(t, int64(0), r.Offset)

	r.ReadToEnd(context.Background())
	assert.Equal(t, int64(aContentLength), r.Offset) // Offset moved to the end of the file
	sink.ExpectNoCalls(t)
}

func TestFileReaderMaxSurgeSizeFileGettingTooLarge(t *testing.T) {
	tempDir := t.TempDir()
	temp := filetest.OpenTemp(t, tempDir)
	f, sink := testFactory(t, withMaxSurgeSize(2*1024*1024)) // 2Mb
	fp, err := f.NewFingerprint(temp)
	require.NoError(t, err)
	r, err := f.NewReader(temp, fp)
	require.NoError(t, err)

	content := strings.Repeat("a", 2*1024) + "\n" // 2Kb
	filetest.WriteString(t, temp, content)
	require.NoError(t, err)
	r.ReadToEnd(context.Background())
	currentExpectedOffset := int64(2*1024 + 1)
	assert.Equal(t, currentExpectedOffset, r.Offset) // Offset moved to the end of the file
	sink.ExpectTokens(t, []byte(strings.Repeat("a", 2*1024)))

	content = strings.Repeat("b", 2*1024*1024) + "\n" // 2Mb
	filetest.WriteString(t, temp, content)
	require.NoError(t, err)
	r.ReadToEnd(context.Background())
	currentExpectedOffset = currentExpectedOffset + int64((2*1024*1024)+1)
	assert.Equal(t, currentExpectedOffset, r.Offset) // Offset moved to the end of the file
	sink.ExpectNoCalls(t)                            // No calls as we hit max surge size

	content = strings.Repeat("c", 2*1024) + "\n" // 2Kb
	filetest.WriteString(t, temp, content)
	require.NoError(t, err)
	r.ReadToEnd(context.Background())
	currentExpectedOffset = currentExpectedOffset + int64(2*1024+1)
	assert.Equal(t, currentExpectedOffset, r.Offset) // Offset moved to the end of the file
	sink.ExpectTokens(t, []byte(strings.Repeat("c", 2*1024)))
}
