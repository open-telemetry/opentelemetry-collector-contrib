// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"io/fs"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnsureTemplatesLoaded(t *testing.T) {
	t.Parallel()

	const (
		rootDir = "templates"
	)

	var (
		templateFiles = map[string]struct{}{
			path.Join(rootDir, "documentation.md.tmpl"):        {},
			path.Join(rootDir, "metrics.go.tmpl"):              {},
			path.Join(rootDir, "metrics_test.go.tmpl"):         {},
			path.Join(rootDir, "config.go.tmpl"):               {},
			path.Join(rootDir, "config_test.go.tmpl"):          {},
			path.Join(rootDir, "readme.md.tmpl"):               {},
			path.Join(rootDir, "status.go.tmpl"):               {},
			path.Join(rootDir, "testdata", "config.yaml.tmpl"): {},
		}
		count = 0
	)
	assert.NoError(t, fs.WalkDir(templateFS, ".", func(path string, d fs.DirEntry, err error) error {
		if d != nil && d.IsDir() {
			return nil
		}
		count++
		assert.Contains(t, templateFiles, path)
		return nil
	}))
	assert.Equal(t, len(templateFiles), count, "Must match the expected number of calls")

}
