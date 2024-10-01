// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/filetest"

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func OpenFile(tb testing.TB, path string) *os.File {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	require.NoError(tb, err)
	tb.Cleanup(func() { _ = file.Close() })
	return file
}

func OpenTemp(t testing.TB, tempDir string) *os.File {
	return OpenTempWithPattern(t, tempDir, "")
}

func ReopenTemp(t testing.TB, name string) *os.File {
	return OpenTempWithPattern(t, filepath.Dir(name), filepath.Base(name))
}

func OpenTempWithPattern(t testing.TB, tempDir, pattern string) *os.File {
	file, err := os.CreateTemp(tempDir, pattern)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func WriteString(t testing.TB, file *os.File, s string) {
	_, err := file.WriteString(s)
	require.NoError(t, err)
}

func TokenWithLength(length int) []byte {
	charset := "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return b
}
