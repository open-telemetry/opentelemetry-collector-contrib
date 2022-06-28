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

package main

import (
	"fmt"
	"path/filepath"
	"runtime"
)

const (
	changelogMD   = "CHANGELOG.md"
	unreleasedDir = "unreleased"
	templateYAML  = "TEMPLATE.yaml"
)

// chlogContext enables tests by allowing them to work in an test directory
type chlogContext struct {
	changelogMD   string
	unreleasedDir string
	templateYAML  string
}

func newChlogContext(rootDir string) chlogContext {
	return chlogContext{
		changelogMD:   filepath.Join(rootDir, changelogMD),
		unreleasedDir: filepath.Join(rootDir, unreleasedDir),
		templateYAML:  filepath.Join(rootDir, unreleasedDir, templateYAML),
	}
}

var defaultCtx = newChlogContext(repoRoot())

func repoRoot() string {
	cmdDir := filepath.Dir(thisDir())
	return filepath.Dir(cmdDir)
}

func thisDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		// This is not expected, but just in case
		fmt.Println("FAIL: Could not determine module directory")
	}
	return filepath.Dir(filename)
}
