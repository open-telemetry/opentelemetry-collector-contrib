// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// CreateTempHostFile create a temporary hostfile
func CreateTempHostFile(t *testing.T, content string) string {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "hosts")

	err := os.WriteFile(tempFile, []byte(content), 0o600)
	require.NoError(t, err)

	return tempFile
}
