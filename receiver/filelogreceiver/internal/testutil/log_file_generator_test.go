// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogFileGenerator(t *testing.T) {
	generator := NewLogFileGenerator(t)

	require.NotNil(t, generator)
	assert.NotNil(t, generator.tb)
	assert.NotNil(t, generator.charset)
	assert.NotNil(t, generator.logLines)
	assert.NotEmpty(t, generator.charset)
	assert.Len(t, generator.logLines, 8)
}

func TestGenerateLogLines(t *testing.T) {
	generator := NewLogFileGenerator(t)

	numLines := 10
	lineLength := 100

	lines := generator.generateLogLines(numLines, lineLength)

	require.Len(t, lines, numLines)

	for _, line := range lines {
		assert.Len(t, line, lineLength)

		// Verify each character is from the charset
		for _, char := range line {
			assert.Contains(t, generator.charset, char)
		}
	}
}

func TestGenerateLogFile(t *testing.T) {
	generator := NewLogFileGenerator(t)

	numLines := 100

	logFilePath := generator.GenerateLogFile(numLines)

	// Verify file was created
	require.NotEmpty(t, logFilePath)

	fileInfo, err := os.Stat(logFilePath)
	require.NoError(t, err)
	assert.False(t, fileInfo.IsDir())
	assert.Greater(t, fileInfo.Size(), int64(0))

	// Read file and verify content
	content, err := os.ReadFile(logFilePath)
	require.NoError(t, err)

	// Count lines (each line ends with \n)
	lineCount := 0
	for _, b := range content {
		if b == '\n' {
			lineCount++
		}
	}

	assert.Equal(t, numLines, lineCount)
}

func TestGenerateLogFileWithDifferentSizes(t *testing.T) {
	tests := []struct {
		name     string
		numLines int
	}{
		{"small file", 10},
		{"medium file", 100},
		{"large file", 1000},
		{"single line", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := NewLogFileGenerator(t)

			logFilePath := generator.GenerateLogFile(tt.numLines)

			fileInfo, err := os.Stat(logFilePath)
			require.NoError(t, err)
			assert.Greater(t, fileInfo.Size(), int64(0))

			content, err := os.ReadFile(logFilePath)
			require.NoError(t, err)

			lineCount := 0
			for _, b := range content {
				if b == '\n' {
					lineCount++
				}
			}

			assert.Equal(t, tt.numLines, lineCount)
		})
	}
}

func TestGenerateLogFileCharsetValidation(t *testing.T) {
	generator := NewLogFileGenerator(t)

	logFilePath := generator.GenerateLogFile(50)

	content, err := os.ReadFile(logFilePath)
	require.NoError(t, err)

	// Verify all characters (except newlines) are from charset
	for _, b := range content {
		if b == '\n' {
			continue
		}
		assert.Contains(t, generator.charset, b)
	}
}
