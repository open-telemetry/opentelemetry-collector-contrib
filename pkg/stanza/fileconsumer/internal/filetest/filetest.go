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
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	require.NoError(tb, err)
	tb.Cleanup(func() { _ = file.Close() })
	return file
}

func OpenTemp(tb testing.TB, tempDir string) *os.File {
	return OpenTempWithPattern(tb, tempDir, "")
}

func ReopenTemp(tb testing.TB, name string) *os.File {
	return OpenTempWithPattern(tb, filepath.Dir(name), filepath.Base(name))
}

func OpenTempWithPattern(tb testing.TB, tempDir, pattern string) *os.File {
	file, err := os.CreateTemp(tempDir, pattern)
	require.NoError(tb, err)
	tb.Cleanup(func() { _ = file.Close() })
	return file
}

func WriteString(tb testing.TB, file *os.File, s string) {
	_, err := file.WriteString(s)
	require.NoError(tb, err)
}

func TokenWithLength(length int) []byte {
	charset := "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return b
}
