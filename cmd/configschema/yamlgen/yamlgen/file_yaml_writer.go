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
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

// fileYAMLWriter is a YAMLWriter implementation that creates yaml files in a collector repo
// source root, one for each component.
type fileYAMLWriter struct {
	dr configschema.DirResolverIntf
}

func (f fileYAMLWriter) write(cfg configschema.CfgInfo, yamlBytes []byte) error {
	path := f.dr.ReflectValueToProjectPath(reflect.ValueOf(cfg.CfgInstance))
	if path == "" {
		return fmt.Errorf("project path not found for component: %s %s", cfg.Group, cfg.Type)
	}
	fp := filepath.Join(path, "cfgschema.yaml")
	err := os.WriteFile(fp, yamlBytes, 0600)
	_, _ = fmt.Printf("wrote file: %v\n", fp)
	if err != nil {
		return fmt.Errorf("error writing yaml file: %v", fp)
	}
	return nil
}

func (f fileYAMLWriter) close() error {
	return nil
}
