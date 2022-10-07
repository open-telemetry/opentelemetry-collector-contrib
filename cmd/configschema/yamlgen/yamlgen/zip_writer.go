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

package yamlgen

import (
	"archive/zip"
	"fmt"
	"path/filepath"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

type zipWriter struct {
	zw *zip.Writer
}

func (z zipWriter) write(cfg configschema.CfgInfo, bytes []byte) error {
	path := filepath.Join(cfg.Group, fmt.Sprintf("%s.yaml", cfg.Type))
	fmt.Printf("added file: %v\n", path)
	writer, err := z.zw.Create(path)
	if err != nil {
		return fmt.Errorf("error creating zip file: %s: %w", path, err)
	}
	_, err = writer.Write(bytes)
	if err != nil {
		return fmt.Errorf("error writing zip: %w", err)
	}
	return nil
}

func (z zipWriter) close() error {
	return z.zw.Close()
}
