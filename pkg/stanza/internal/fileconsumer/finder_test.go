// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileconsumer

import (
	"io/ioutil"
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
			files:    []string{"a/1.log", "a/2.log", "b/1.log", "b/2.log"},
			include:  []string{"a/*.log", "b/*.log"},
			exclude:  []string{},
			expected: []string{"a/1.log", "a/2.log", "b/1.log", "b/2.log"},
		},
		{
			name:     "IncludeMultipleDirectoriesVaryingDepth",
			files:    []string{"1.log", "a/1.log", "a/b/1.log", "c/1.log"},
			include:  []string{"*.log", "a/*.log", "a/b/*.log", "c/*.log"},
			exclude:  []string{},
			expected: []string{"1.log", "a/1.log", "a/b/1.log", "c/1.log"},
		},
		{
			name:     "DoubleStarSameDepth",
			files:    []string{"a/1.log", "b/1.log", "c/1.log"},
			include:  []string{"**/*.log"},
			exclude:  []string{},
			expected: []string{"a/1.log", "b/1.log", "c/1.log"},
		},
		{
			name:     "DoubleStarVaryingDepth",
			files:    []string{"1.log", "a/1.log", "a/b/1.log", "c/1.log"},
			include:  []string{"**/*.log"},
			exclude:  []string{},
			expected: []string{"1.log", "a/1.log", "a/b/1.log", "c/1.log"},
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
				require.NoError(t, ioutil.WriteFile(f, []byte(filepath.Base(f)), 0000))
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
