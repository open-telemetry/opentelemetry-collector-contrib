// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Deprecated: [v0.92.0] This package is deprecated and will be removed in a future release.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30187
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
// Deprecated: [v0.92.0] This package is deprecated and will be removed in a future release.
// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30187
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
		return fmt.Errorf("error reading fields for component %v/%v: %w", cfg.Group, cfg.Type, err)
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
