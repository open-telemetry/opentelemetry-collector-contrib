// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage/internal/utils"

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncat(t *testing.T) {
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
			expectedErr: "file name too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			file, err := os.OpenFile(filepath.Join(tempDir, tt.input), os.O_RDWR|os.O_CREATE, 0644)
			if tt.expectedErr != "" {
				if !strings.Contains(err.Error(), tt.expectedErr) {
					require.ErrorContains(t, err, tt.expectedErr)
				}
				require.Nil(t, file)
				truncated := Truncate(tt.input)
				file, err = os.OpenFile(filepath.Join(tempDir, truncated), os.O_RDWR|os.O_CREATE, 0644)
				require.NoError(t, err)
				require.NotNil(t, file)
			} else {
				require.NoError(t, err)
				require.NotNil(t, file)
			}

		})
	}
}
