// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filereloadereceiver

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/mitchellh/hashstructure/v2"
)

type fileWatcher struct {
	path      string
	lastHash  uint64
	lastCheck time.Time
}

func newFileWatcher(path string) *fileWatcher {
	return &fileWatcher{
		path: path,
	}
}

func (f *fileWatcher) List() ([]string, bool, error) {
	defer func() {
		f.lastCheck = time.Now()
	}()
	paths, err := filepath.Glob(f.path)
	if err != nil {
		return nil, false, err
	}

	var (
		files   = make([]string, 0, len(paths))
		updated = false
	)
	for _, path := range paths {
		fi, err := os.Stat(path)
		if err != nil {
			continue // skip through non-existent paths
		}

		if !fi.Mode().IsRegular() {
			continue // skip anything that is not a file
		}

		if fi.ModTime().After(f.lastCheck) {
			updated = true // file has changed since last check
		}
		files = append(files, path)
	}

	sort.Strings(files)
	hash, err := hashstructure.Hash(files, hashstructure.FormatV2, nil)
	if err != nil {
		return files, updated, err
	}

	defer func() {
		f.lastHash = hash
	}()
	if updated || hash != f.lastHash {
		return files, true, nil
	}

	return files, false, nil
}
