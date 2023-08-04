// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package finder

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name        string
		globs       []string
		expectedErr string
	}{
		{
			name:  "Empty",
			globs: []string{},
		},
		{
			name:  "Single",
			globs: []string{"*.log"},
		},
		{
			name:  "Multiple",
			globs: []string{"*.log", "*.txt"},
		},
		{
			name:        "Invalid",
			globs:       []string{"[a-z"},
			expectedErr: "parse glob: syntax error in pattern",
		},
		{
			name:        "ValidAndInvalid",
			globs:       []string{"*.log", "[a-z"},
			expectedErr: "parse glob: syntax error in pattern",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.globs)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFindFiles(t *testing.T) {
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
			include:  []string{"*1*", "a*", "*.log"},
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
			files:    []string{"a1.log", "a2.txt", filepath.Join("b", "b1.log"), filepath.Join("b", "b2.log")},
			include:  []string{"*"},
			expected: []string{"a1.log", "a2.txt"},
		},
		{
			name:     "MultiLevelFilesOnly",
			files:    []string{"a1.log", "a2.txt", filepath.Join("b", "b1.log"), filepath.Join("b", "b2.txt"), filepath.Join("b", "c", "c1.csv")},
			include:  []string{filepath.Join("**", "*")},
			expected: []string{"a1.log", "a2.txt", filepath.Join("b", "b1.log"), filepath.Join("b", "b2.txt"), filepath.Join("b", "c", "c1.csv")},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cwd, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(t.TempDir()))
			defer func() {
				require.NoError(t, os.Chdir(cwd))
			}()
			for _, f := range tc.files {
				require.NoError(t, os.MkdirAll(filepath.Dir(f), 0700))

				file, err := os.OpenFile(f, os.O_CREATE|os.O_RDWR, 0600)
				require.NoError(t, err)

				_, err = file.WriteString(filepath.Base(f))
				require.NoError(t, err)
				require.NoError(t, file.Close())
			}
			assert.Equal(t, tc.expected, FindFiles(tc.include, tc.exclude))
		})
	}
}
