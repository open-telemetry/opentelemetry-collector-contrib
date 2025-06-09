// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fingerprint

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/filetest"
)

func TestNewDoesNotModifyOffset(t *testing.T) {
	fingerprint := "this is the fingerprint"
	next := "this comes after the fingerprint and is substantially longer than the fingerprint"
	extra := "fin"

	fileContents := fmt.Sprintf("%s%s%s\n", fingerprint, next, extra)

	tempDir := t.TempDir()
	temp, err := os.CreateTemp(tempDir, "")
	require.NoError(t, err)
	defer temp.Close()

	_, err = temp.WriteString(fileContents)
	require.NoError(t, err)

	// Validate that the file is actually the expected size after writing
	info, err := temp.Stat()
	require.NoError(t, err)
	require.Equal(t, len(fileContents), int(info.Size()))

	// Set the file descriptors pointer to the beginning of the file
	_, err = temp.Seek(0, 0)
	require.NoError(t, err)

	fp, err := NewFromFile(temp, len(fingerprint), false)
	require.NoError(t, err)

	// Validate the fingerprint is the correct size
	require.Len(t, fp.firstBytes, len(fingerprint))

	// Validate that reading the fingerprint did not adjust the
	// file descriptor's internal offset (as using Seek does)
	allButExtra := make([]byte, len(fingerprint)+len(next))
	n, err := temp.Read(allButExtra)
	require.NoError(t, err)
	require.Equal(t, len(allButExtra), n)
	require.Equal(t, fileContents[:len(allButExtra)], string(allButExtra))
}

func TestNew(t *testing.T) {
	fp := New([]byte("hello"))
	require.Equal(t, []byte("hello"), fp.firstBytes)
}

func TestNewFromFile(t *testing.T) {
	cases := []struct {
		name            string
		fingerprintSize int
		fileSize        int
		expectedLen     int
	}{
		{
			name:            "defaultExactFileSize",
			fingerprintSize: DefaultSize,
			fileSize:        DefaultSize,
			expectedLen:     DefaultSize,
		},
		{
			name:            "defaultWithFileHalfOfFingerprint",
			fingerprintSize: DefaultSize,
			fileSize:        DefaultSize / 2,
			expectedLen:     DefaultSize / 2,
		},
		{
			name:            "defaultWithFileTwiceFingerprint",
			fingerprintSize: DefaultSize,
			fileSize:        DefaultSize * 2,
			expectedLen:     DefaultSize,
		},
		{
			name:            "minFingerprintExactFileSize",
			fingerprintSize: MinSize,
			fileSize:        MinSize,
			expectedLen:     MinSize,
		},
		{
			name:            "minFingerprintWithSmallerFileSize",
			fingerprintSize: MinSize,
			fileSize:        MinSize / 2,
			expectedLen:     MinSize / 2,
		},
		{
			name:            "minFingerprintWithLargerFileSize",
			fingerprintSize: MinSize,
			fileSize:        DefaultSize,
			expectedLen:     MinSize,
		},
		{
			name:            "largeFingerprintSmallFile",
			fingerprintSize: 1024 * 1024,
			fileSize:        1024,
			expectedLen:     1024,
		},
		{
			name:            "largeFingerprintLargeFile",
			fingerprintSize: 1024 * 8,
			fileSize:        1024 * 128,
			expectedLen:     1024 * 8,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tempDir := t.TempDir()
			temp, err := os.CreateTemp(tempDir, "")
			require.NoError(t, err)
			defer temp.Close()

			_, err = temp.WriteString(string(tokenWithLength(tc.fileSize)))
			require.NoError(t, err)

			// Validate that the file is actually the expected size after writing
			info, err := temp.Stat()
			require.NoError(t, err)
			require.Equal(t, tc.fileSize, int(info.Size()))

			fp, err := NewFromFile(temp, tc.fingerprintSize, false)
			require.NoError(t, err)

			require.Len(t, fp.firstBytes, tc.expectedLen)
		})
	}
}

func TestCopy(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"hello",
		"asdfsfaddsfas",
		string(tokenWithLength(MinSize)),
		string(tokenWithLength(DefaultSize)),
		string(tokenWithLength(1234)),
	}

	for _, tc := range cases {
		fp := New([]byte(tc))

		cp := fp.Copy()

		// Did not change original
		require.Equal(t, tc, string(fp.firstBytes))

		// Copy is also good
		require.Equal(t, tc, string(cp.firstBytes))

		// Modify copy
		cp.firstBytes = append(cp.firstBytes, []byte("also")...)

		// Still did not change original
		require.Equal(t, tc, string(fp.firstBytes))

		// Copy is modified
		require.Equal(t, tc+"also", string(cp.firstBytes))
	}
}

func TestEqual(t *testing.T) {
	empty := New([]byte(""))
	empty2 := New([]byte(""))
	hello := New([]byte("hello"))
	hello2 := New([]byte("hello"))
	world := New([]byte("world"))
	world2 := New([]byte("world"))
	helloworld := New([]byte("helloworld"))
	helloworld2 := New([]byte("helloworld"))

	require.True(t, empty.Equal(empty2))
	require.True(t, hello.Equal(hello2))
	require.True(t, world.Equal(world2))
	require.True(t, helloworld.Equal(helloworld2))

	require.False(t, hello.Equal(empty))
	require.False(t, empty.Equal(hello))

	require.False(t, hello.Equal(world))
	require.False(t, world.Equal(hello))

	require.False(t, hello.Equal(helloworld))
	require.False(t, helloworld.Equal(hello))
}

func TestStartsWith(t *testing.T) {
	empty := New([]byte(""))
	hello := New([]byte("hello"))
	world := New([]byte("world"))
	helloworld := New([]byte("helloworld"))

	// Empty never matches
	require.False(t, hello.StartsWith(empty))
	require.False(t, empty.StartsWith(hello))

	require.True(t, hello.StartsWith(hello))
	require.False(t, hello.StartsWith(helloworld))

	require.True(t, helloworld.StartsWith(hello))
	require.True(t, helloworld.StartsWith(helloworld))
	require.False(t, helloworld.StartsWith(world))
}

// Generates a file filled with many random bytes, then
// writes the same bytes to a second file, one byte at a time.
// Validates, after each byte is written, that fingerprint
// matching would successfully associate the two files.
// The static file can be thought of as the present state of
// the file, while each iteration of the growing file represents
// a possible state of the same file at a previous time.
func TestStartsWith_FromFile(t *testing.T) {
	r := rand.New(rand.NewPCG(112, 358))
	fingerprintSize := 10
	fileLength := 12 * fingerprintSize
	fillRandomBytes := func(buf []byte) {
		// TODO: when we upgrade to go1.23,
		// use rand.ChaCha8.Read.
		//
		// NOTE: we can cheat here since know the
		// buffer length is a multiple of 4, due to
		// fileLength being a multiple of 12.
		for i := range len(buf) / 4 {
			binary.BigEndian.PutUint32(
				buf[i*4:(i+1)*4],
				r.Uint32(),
			)
		}
	}

	tempDir := t.TempDir()

	// Make a []byte we can write one at a time
	content := make([]byte, fileLength)
	fillRandomBytes(content)

	// Overwrite some bytes with \n to ensure
	// we are testing a file with multiple lines
	newlineMask := make([]byte, fileLength)
	fillRandomBytes(newlineMask)
	for i, b := range newlineMask {
		if b == 0 && i != 0 { // 1/256 chance, but never first byte
			content[i] = byte('\n')
		}
	}

	fullFile, err := os.CreateTemp(tempDir, "")
	require.NoError(t, err)
	defer fullFile.Close()

	_, err = fullFile.Write(content)
	require.NoError(t, err)

	fff, err := NewFromFile(fullFile, fingerprintSize, false)
	require.NoError(t, err)

	partialFile, err := os.CreateTemp(tempDir, "")
	require.NoError(t, err)
	defer partialFile.Close()

	// Write the first byte before comparing, since empty files will never match
	_, err = partialFile.Write(content[:1])
	require.NoError(t, err)
	content = content[1:]

	// Write one byte at a time and validate that
	// full fingerprint still starts with updated partial
	for i := range content {
		_, err = partialFile.Write(content[i:i])
		require.NoError(t, err)

		pff, err := NewFromFile(partialFile, fingerprintSize, false)
		require.NoError(t, err)

		require.True(t, fff.StartsWith(pff))
	}
}

func tokenWithLength(length int) []byte {
	charset := "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return b
}

func TestMarshalUnmarshal(t *testing.T) {
	fp := New([]byte("hello"))
	b, err := fp.MarshalJSON()
	require.NoError(t, err)

	fp2 := new(Fingerprint)
	require.NoError(t, fp2.UnmarshalJSON(b))

	require.Equal(t, fp, fp2)
}

// Test compressed and uncompressed file with same content have equal fingerprint
func TestCompressionFingerprint(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(DecompressedFingerprintFeatureGate.ID(), true))
	tmp := t.TempDir()
	compressedFile := filetest.OpenTempWithPattern(t, tmp, "*.gz")
	gzipWriter := gzip.NewWriter(compressedFile)
	defer gzipWriter.Close()

	data := []byte("this is a first test line")
	// Write data
	n, err := gzipWriter.Write(data)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())
	require.NotZero(t, n, "gzip file should not be empty")

	// set seek to the start of the file
	_, err = compressedFile.Seek(0, io.SeekStart)
	require.NoError(t, err)

	compressedFP, err := NewFromFile(compressedFile, len(data), true)
	require.NoError(t, err)

	uncompressedFP := New(data)
	uncompressedFP.Equal(compressedFP)
}
