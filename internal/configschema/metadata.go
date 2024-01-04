// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package configschema // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/configschema"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/component"
	"gopkg.in/yaml.v2"
)

// GenerateMetadata generates the metadata of a component.
func GenerateMetadata(f component.Factory, sourceDir string, outputDir string) error {
	writer := newMetadataFileWriter(outputDir)
	var cfg CfgInfo
	var err error
	if cfg, err = GetCfgInfo(f); err != nil {
		return err
	}
	if err = writeComponentYAML(writer, cfg, sourceDir); err != nil {
		return err
	}
	return nil
}

func writeComponentYAML(yw metadataWriter, cfg CfgInfo, srcRoot string) error {
	fields, err := ReadFields(reflect.ValueOf(cfg.CfgInstance), srcRoot)
	if err != nil {
		return fmt.Errorf("error reading fields for component %v: %w", cfg.Type, err)
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
