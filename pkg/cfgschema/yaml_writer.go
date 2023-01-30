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

package cfgschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/cfgschema"

import (
	"fmt"
	"os"
	"path/filepath"
)

type writeYamlFunc func(cfg cfgInfo, bytes []byte) error

type yamlFileWriter struct {
	baseDir     string
	dirsCreated map[string]struct{}
}

func newYamlFileWriter(dir string) *yamlFileWriter {
	return &yamlFileWriter{
		dirsCreated: map[string]struct{}{},
		baseDir:     dir,
	}
}

func (w *yamlFileWriter) writeFile(cfg cfgInfo, yamlBytes []byte) error {
	groupDir := filepath.Join(w.baseDir, cfg.Group)
	if err := w.prepDir(groupDir); err != nil {
		return err
	}
	filename := filepath.Join(groupDir, fmt.Sprintf("%s.yaml", cfg.Type))
	fmt.Printf("writing file: %s\n", filename)
	return os.WriteFile(filename, yamlBytes, 0600)
}

func (w *yamlFileWriter) prepDir(dir string) error {
	if _, ok := w.dirsCreated[dir]; !ok {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to make dir %q: %w", dir, err)
		}
		w.dirsCreated[dir] = struct{}{}
	}
	return nil
}
