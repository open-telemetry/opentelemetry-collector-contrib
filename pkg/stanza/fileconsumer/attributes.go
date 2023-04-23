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

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"path/filepath"
	"runtime"

	"go.uber.org/multierr"
)

type FileAttributes struct {
	Name             string `json:"-"`
	Path             string `json:"-"`
	NameResolved     string `json:"-"`
	PathResolved     string `json:"-"`
	HeaderAttributes map[string]any
}

// HeaderAttributesCopy gives a copy of the HeaderAttributes, in order to restrict mutation of the HeaderAttributes.
func (f *FileAttributes) HeaderAttributesCopy() map[string]any {
	return mapCopy(f.HeaderAttributes)
}

// resolveFileAttributes resolves file attributes
// and sets it to empty string in case of error
func resolveFileAttributes(path string) (*FileAttributes, error) {
	resolved := ""
	var symErr error
	// Dirty solution, waiting for this permanent fix https://github.com/golang/go/issues/39786
	// EvalSymlinks on windows is partially working depending on the way you use Symlinks and Junctions
	if runtime.GOOS != "windows" {
		resolved, symErr = filepath.EvalSymlinks(path)
	} else {
		resolved = path
	}
	abs, absErr := filepath.Abs(resolved)

	return &FileAttributes{
		Path:         path,
		Name:         filepath.Base(path),
		PathResolved: abs,
		NameResolved: filepath.Base(abs),
	}, multierr.Combine(symErr, absErr)
}
