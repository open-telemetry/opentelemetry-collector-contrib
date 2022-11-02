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

package cfgmetadatagen

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

// YAMLWriter is an interface whose implementations generate either YAML files
// or YAML file entries in a zip file.
type YAMLWriter interface {
	write(cfg configschema.CfgInfo, yamlBytes []byte) error
	close() error
}

// NewYAMLWriter creates a YAMLWriter implementation based on the value of isZip.
func NewYAMLWriter(dr configschema.DirResolver, outputDir string) (YAMLWriter, error) {
	if outputDir != "" {
		return newCreateDirsYAMLWriter(outputDir)
	}
	return newSourceTreeYAMLWriter(dr)
}

func newCreateDirsYAMLWriter(dir string) (YAMLWriter, error) {
	return &createDirsYAMLWriter{
		dirsCreated: map[string]struct{}{},
		baseDir:     dir,
	}, nil
}

func newSourceTreeYAMLWriter(dr configschema.DirResolver) (YAMLWriter, error) {
	return sourceTreeYAMLWriter{dr: dr}, nil
}
