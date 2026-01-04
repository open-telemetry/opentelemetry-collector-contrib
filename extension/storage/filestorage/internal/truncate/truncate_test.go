// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package truncate // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage/internal/truncate"

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncate(t *testing.T) {
	longNameErr := "file name too long"
	if runtime.GOOS == "windows" {
		longNameErr = "The filename, directory name, or volume label syntax is incorrect"
	}
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:  "short name",
			input: "short_filename.txt",
		},
		{
			name:  "exactly max length",
			input: strings.Repeat("a", 255),
		},
		{
			name:        "exceeds max length",
			input:       strings.Repeat("b", 1000),
			expectedErr: longNameErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			file, err := os.OpenFile(filepath.Join(tempDir, tt.input), os.O_RDWR|os.O_CREATE, 0o644)
			if tt.expectedErr != "" {
				if !strings.Contains(err.Error(), tt.expectedErr) {
					require.ErrorContains(t, err, tt.expectedErr)
				}
				require.Nil(t, file)
				truncated := Truncate(tt.input)
				file, err = os.OpenFile(filepath.Join(tempDir, truncated), os.O_RDWR|os.O_CREATE, 0o644)
				require.NoError(t, err)
				require.NotNil(t, file)
				require.NoError(t, file.Close())
			} else {
				require.NoError(t, err)
				require.NotNil(t, file)
				require.NoError(t, file.Close())
			}
		})
	}
}
