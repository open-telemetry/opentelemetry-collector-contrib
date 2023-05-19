// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileconsumer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		files    []string
		include  []string
		exclude  []string
		expected []string
	}{
		{
			name:     "IncludeOne",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"a1.log"},
			exclude:  []string{},
			expected: []string{"a1.log"},
		},
		{
			name:     "IncludeNone",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"c*.log"},
			exclude:  []string{},
			expected: []string{},
		},
		{
			name:     "IncludeAll",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"*"},
			exclude:  []string{},
			expected: []string{"a1.log", "a2.log", "b1.log", "b2.log"},
		},
		{
			name:     "IncludeLogs",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"*.log"},
			exclude:  []string{},
			expected: []string{"a1.log", "a2.log", "b1.log", "b2.log"},
		},
		{
			name:     "IncludeA",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"a*.log"},
			exclude:  []string{},
			expected: []string{"a1.log", "a2.log"},
		},
		{
			name:     "Include2s",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"*2.log"},
			exclude:  []string{},
			expected: []string{"a2.log", "b2.log"},
		},
		{
			name:     "Exclude",
			files:    []string{"include.log", "exclude.log"},
			include:  []string{"*"},
			exclude:  []string{"exclude.log"},
			expected: []string{"include.log"},
		},
		{
			name:     "ExcludeMany",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"*"},
			exclude:  []string{"a*.log", "*2.log"},
			expected: []string{"b1.log"},
		},
		{
			name:     "ExcludeDuplicates",
			files:    []string{"a1.log", "a2.log", "b1.log", "b2.log"},
			include:  []string{"*1*", "a*"},
			exclude:  []string{"a*.log", "*2.log"},
			expected: []string{"b1.log"},
		},
		{
			name:     "IncludeMultipleDirectories",
			files:    []string{filepath.Join("a", "1.log"), filepath.Join("a", "2.log"), filepath.Join("b", "1.log"), filepath.Join("b", "2.log")},
			include:  []string{filepath.Join("a", "*.log"), filepath.Join("b", "*.log")},
			exclude:  []string{},
			expected: []string{filepath.Join("a", "1.log"), filepath.Join("a", "2.log"), filepath.Join("b", "1.log"), filepath.Join("b", "2.log")},
		},
		{
			name:     "IncludeMultipleDirectoriesVaryingDepth",
			files:    []string{"1.log", filepath.Join("a", "1.log"), filepath.Join("a", "b", "1.log"), filepath.Join("c", "1.log")},
			include:  []string{"*.log", filepath.Join("a", "*.log"), filepath.Join("a", "b", "*.log"), filepath.Join("c", "*.log")},
			exclude:  []string{},
			expected: []string{"1.log", filepath.Join("a", "1.log"), filepath.Join("a", "b", "1.log"), filepath.Join("c", "1.log")},
		},
		{
			name:     "DoubleStarSameDepth",
			files:    []string{filepath.Join("a", "1.log"), filepath.Join("b", "1.log"), filepath.Join("c", "1.log")},
			include:  []string{filepath.Join("**", "*.log")},
			exclude:  []string{},
			expected: []string{filepath.Join("a", "1.log"), filepath.Join("b", "1.log"), filepath.Join("c", "1.log")},
		},
		{
			name:     "DoubleStarVaryingDepth",
			files:    []string{"1.log", filepath.Join("a", "1.log"), filepath.Join("a", "b", "1.log"), filepath.Join("c", "1.log")},
			include:  []string{filepath.Join("**", "*.log")},
			exclude:  []string{},
			expected: []string{"1.log", filepath.Join("a", "1.log"), filepath.Join("a", "b", "1.log"), filepath.Join("c", "1.log")},
		},
		{
			name:     "SingleLevelFilesOnly",
			files:    []string{"a1.log", "a2.txt", "b/b1.log", "b/b2.txt"},
			include:  []string{"*"},
			expected: []string{"a1.log", "a2.txt"},
		},
		{
			name:     "MultiLevelFilesOnly",
			files:    []string{"a1.log", "a2.txt", "b/b1.log", "b/b2.txt", "b/c/c1.csv"},
			include:  []string{filepath.Join("**", "*")},
			expected: []string{"a1.log", "a2.txt", filepath.Join("b", "b1.log"), filepath.Join("b", "b2.txt"), filepath.Join("b", "c", "c1.csv")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			files := absPath(tempDir, tc.files)
			include := absPath(tempDir, tc.include)
			exclude := absPath(tempDir, tc.exclude)
			expected := absPath(tempDir, tc.expected)

			for _, f := range files {
				require.NoError(t, os.MkdirAll(filepath.Dir(f), 0700))
				require.NoError(t, os.WriteFile(f, []byte(filepath.Base(f)), 0000))
			}

			finder := Finder{include, exclude}
			require.ElementsMatch(t, finder.FindFiles(), expected)
		})
	}
}

func absPath(tempDir string, files []string) []string {
	absFiles := make([]string, 0, len(files))
	for _, f := range files {
		absFiles = append(absFiles, filepath.Join(tempDir, f))
	}
	return absFiles
}
