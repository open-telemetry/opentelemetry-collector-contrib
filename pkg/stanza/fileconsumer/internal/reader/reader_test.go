// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"
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
	require.Equal(t, []byte("testlog1\n"), reader.Fingerprint.FirstBytes)
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

			f, _ := testFactory(t, withSinkBufferSize(3*fpSize/lineLen), withFingerprintSize(fpSize))
			fp, err := f.NewFingerprint(temp)
			require.NoError(t, err)
			require.Equal(t, []byte(""), fp.FirstBytes)

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
				require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)
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
	lineLens := []int{3, 5, 7, 11, 13, 17, 19, 23, 27}

	for _, lineLen := range lineLens {
		lineLen := lineLen
		t.Run(fmt.Sprintf("%d", lineLen), func(t *testing.T) {
			t.Parallel()

			f, _ := testFactory(t, withSinkBufferSize(3*fpSize/lineLen), withFingerprintSize(fpSize))

			tempDir := t.TempDir()
			temp := filetest.OpenTemp(t, tempDir)

			fp, err := f.NewFingerprint(temp)
			require.NoError(t, err)
			require.Equal(t, []byte(""), fp.FirstBytes)

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
				require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)
			}

			// Recreate the factory with a larger fingerprint size
			f, _ = testFactory(t, withSinkBufferSize(3*fpSize/lineLen), withFingerprintSize(fpSize*lineLen/3))

			// Recreate the reader with the new factory
			reader, err = f.NewReaderFromMetadata(filetest.OpenFile(t, temp.Name()), reader.Close())
			require.NoError(t, err)

			line := string(filetest.TokenWithLength(lineLen-1)) + "\n"
			fileContent = append(fileContent, []byte(line)...)

			filetest.WriteString(t, temp, line)
			reader.ReadToEnd(context.Background())
			require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)

			// Recreate the factory with a smaller fingerprint size
			f, _ = testFactory(t, withSinkBufferSize(3*fpSize/lineLen), withFingerprintSize(fpSize/2))

			// Recreate the reader with the new factory
			reader, err = f.NewReaderFromMetadata(filetest.OpenFile(t, temp.Name()), reader.Close())
			require.NoError(t, err)

			line = string(filetest.TokenWithLength(lineLen-1)) + "\n"
			fileContent = append(fileContent, []byte(line)...)

			filetest.WriteString(t, temp, line)
			reader.ReadToEnd(context.Background())
			require.Equal(t, fileContent[:expectedFP], reader.Fingerprint.FirstBytes)
		})
	}
}
