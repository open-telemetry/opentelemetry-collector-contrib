// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fingerprint

import (
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
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

	fp, err := New(temp, len(fingerprint))
	require.NoError(t, err)

	// Validate the fingerprint is the correct size
	require.Equal(t, len(fingerprint), len(fp.FirstBytes))

	// Validate that reading the fingerprint did not adjust the
	// file descriptor's internal offset (as using Seek does)
	allButExtra := make([]byte, len(fingerprint)+len(next))
	n, err := temp.Read(allButExtra)
	require.NoError(t, err)
	require.Equal(t, len(allButExtra), n)
	require.Equal(t, fileContents[:len(allButExtra)], string(allButExtra))
}

func TestNew(t *testing.T) {
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

			fp, err := New(temp, tc.fingerprintSize)
			require.NoError(t, err)

			require.Equal(t, tc.expectedLen, len(fp.FirstBytes))
		})
	}
}

func TestFingerprintCopy(t *testing.T) {
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
		fp := &Fingerprint{FirstBytes: []byte(tc)}

		cp := fp.Copy()

		// Did not change original
		require.Equal(t, tc, string(fp.FirstBytes))

		// Copy is also good
		require.Equal(t, tc, string(cp.FirstBytes))

		// Modify copy
		cp.FirstBytes = append(cp.FirstBytes, []byte("also")...)

		// Still did not change original
		require.Equal(t, tc, string(fp.FirstBytes))

		// Copy is modified
		require.Equal(t, tc+"also", string(cp.FirstBytes))
	}
}

func TestEqual(t *testing.T) {
	empty := &Fingerprint{FirstBytes: []byte("")}
	empty2 := &Fingerprint{FirstBytes: []byte("")}
	hello := &Fingerprint{FirstBytes: []byte("hello")}
	hello2 := &Fingerprint{FirstBytes: []byte("hello")}
	world := &Fingerprint{FirstBytes: []byte("world")}
	world2 := &Fingerprint{FirstBytes: []byte("world")}
	helloworld := &Fingerprint{FirstBytes: []byte("helloworld")}
	helloworld2 := &Fingerprint{FirstBytes: []byte("helloworld")}

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
	empty := &Fingerprint{FirstBytes: []byte("")}
	hello := &Fingerprint{FirstBytes: []byte("hello")}
	world := &Fingerprint{FirstBytes: []byte("world")}
	helloworld := &Fingerprint{FirstBytes: []byte("helloworld")}

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
	r := rand.New(rand.NewSource(112358))
	fingerprintSize := 10
	fileLength := 12 * fingerprintSize

	tempDir := t.TempDir()

	// Make a []byte we can write one at a time
	content := make([]byte, fileLength)
	r.Read(content) // Fill slice with random bytes

	// Overwrite some bytes with \n to ensure
	// we are testing a file with multiple lines
	newlineMask := make([]byte, fileLength)
	r.Read(newlineMask) // Fill slice with random bytes
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

	fff, err := New(fullFile, fingerprintSize)
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

		pff, err := New(partialFile, fingerprintSize)
		require.NoError(t, err)

		require.True(t, fff.StartsWith(pff))
	}
}

func tokenWithLength(length int) []byte {
	charset := "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return b
}
