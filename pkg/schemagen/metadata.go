// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package schemagen // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/schemagen"

import (
	"os"
	"path"

	"gopkg.in/yaml.v3"
)

type metadata struct {
	Type   string `mapstructure:"type"`
	Status struct {
		Class string `mapstructure:"class"`
	} `mapstructure:"status"`
	Parent string `mapstructure:"parent"`
}

func readMetadata(dir string) (*metadata, bool) {
	mdPath := path.Join(dir, "metadata.yaml")
	if data, err := os.ReadFile(mdPath); err == nil {
		var m metadata
		if err := yaml.Unmarshal(data, &m); err == nil {
			return &m, true
		}
	}
	return nil, false
}
