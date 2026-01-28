// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	SettingsFileName = ".schemagen.yaml"
)

type (
	Settings struct {
		Mappings           Mappings           `yaml:"mappings"`
		ComponentOverrides ComponentOverrides `yaml:"componentOverrides"`
		AllowedRefs        []string           `yaml:"allowedRefs"`
	}
	Mappings           map[string]PackagesMapping
	PackagesMapping    map[string]TypeDesc
	ComponentOverrides map[string]ComponentOverride
	ComponentOverride  struct {
		ConfigName string `yaml:"configName"`
	}
	TypeDesc struct {
		SchemaType     SchemaType `yaml:"schemaType"`
		Format         string     `yaml:"format"`
		SkipAnnotation bool       `yaml:"skipAnnotation"`
	}
)

func ReadSettingsFile() (*Settings, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, false
	}

	for {
		candidate := filepath.Join(dir, SettingsFileName)
		if data, err := os.ReadFile(candidate); err == nil {
			var s Settings
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
