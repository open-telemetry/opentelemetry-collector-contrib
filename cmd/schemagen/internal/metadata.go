// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type Metadata struct {
	Type   string `mapstructure:"type"`
	Status struct {
		Class string `mapstructure:"class"`
	} `mapstructure:"status"`
	Parent string `mapstructure:"parent"`
}

func ReadMetadata(dir string) (*Metadata, bool) {
	mdPath := path.Join(dir, "metadata.yaml")
	if data, err := os.ReadFile(mdPath); err == nil {
		var m Metadata
		if err := yaml.Unmarshal(data, &m); err == nil {
			return &m, true
		}
	}
	return nil, false
}
