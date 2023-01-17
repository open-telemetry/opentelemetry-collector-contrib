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
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/otelcol"
	"gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

// GenerateFiles is the entry point for cfgmetadatagen. Component factories are
// passed in so it can be used by other distros.
func GenerateFiles(factories otelcol.Factories, sourceDir string, outputDir string) error {
	dr := configschema.NewDirResolver(sourceDir, configschema.DefaultModule)
	writer := newMetadataFileWriter(outputDir)
	configs := configschema.GetAllCfgInfos(factories)
	for _, cfg := range configs {
		err := writeComponentYAML(writer, cfg, dr)
		if err != nil {
			fmt.Printf("skipped writing config meta yaml: %v\n", err)
		}
	}
	return nil
}

func writeComponentYAML(yw metadataWriter, cfg configschema.CfgInfo, dr configschema.DirResolver) error {
	fields, err := configschema.ReadFields(reflect.ValueOf(cfg.CfgInstance), dr)
	if err != nil {
		return fmt.Errorf("error reading fields for component: %w", err)
	}
	yamlBytes, err := yaml.Marshal(fields)
	if err != nil {
		return fmt.Errorf("error marshaling to yaml: %w", err)
	}
	err = yw.write(cfg, yamlBytes)
	if err != nil {
		return fmt.Errorf("error writing component yaml: %w", err)
	}
	return nil
}
