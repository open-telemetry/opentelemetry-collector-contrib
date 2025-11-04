// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestIsGzipFile(t *testing.T) {
	temp, err := os.Create(filepath.Join(t.TempDir(), "test.log"))
	require.NoError(t, err)

	tempWrite := gzip.NewWriter(temp)
	_, err = tempWrite.Write([]byte("this is test data and the header should prove this is gzip"))
	require.NoError(t, err)
	tempWrite.Close()

	// set offset to start
	_, err = temp.Seek(0, io.SeekStart)
	require.NoError(t, err)

	require.True(t, IsGzipFile(temp, zap.NewNop()))
}
