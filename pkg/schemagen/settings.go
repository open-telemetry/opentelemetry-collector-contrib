// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemagen // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/schemagen"

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const settingsFileName = ".schemagen.yaml"

type (
	settings struct {
		Namespace          string             `yaml:"namespace"`
		Mappings           mappings           `yaml:"mappings"`
		ComponentOverrides componentOverrides `yaml:"componentOverrides"`
		AllowedRefs        []string           `yaml:"allowedRefs"`
	}
	mappings           map[string]packagesMapping
	packagesMapping    map[string]typeDesc
	componentOverrides map[string]componentOverride
	componentOverride  struct {
		ConfigName string `yaml:"configName"`
	}
	typeDesc struct {
		SchemaType     schemaType `yaml:"schemaType"`
		Format         string     `yaml:"format"`
		SkipAnnotation bool       `yaml:"skipAnnotation"`
	}
)

func readSettingsFile() (*settings, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, false
	}

	for {
		candidate := filepath.Join(dir, settingsFileName)
		if data, err := os.ReadFile(candidate); err == nil {
			var s settings
			if err := yaml.Unmarshal(data, &s); err == nil {
				fmt.Println("Settings file read from: ", candidate)
				return &s, true
			}
			fmt.Println("Warning: failed to parse config file:", candidate)
			return nil, false
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, false
}
