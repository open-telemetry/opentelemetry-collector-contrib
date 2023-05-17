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

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
)

func TestComponentsArePresent(t *testing.T) {
	components, err := components()
	require.NoError(t, err)
	var metadataComponents []string
	e := filepath.Walk(filepath.Join("..", ".."), func(path string, info os.FileInfo, err error) error {
		if err == nil && "metadata.yaml" == info.Name() {
			metadataComponents = append(metadataComponents, path)
		}
		return nil
	})
	require.NoError(t, e)

	for _, metadataComponent := range metadataComponents {
		t.Run(metadataComponent, func(tt *testing.T) {
			m, err := loadMetadata(metadataComponent)
			require.NoError(tt, err)
			if m.Status == nil {
				tt.Skip("no status present, skipping", metadataComponent)
				return
			}
			inDevelopment := true
			for stability, pipelines := range m.Status.Stability {
				if stability != "development" && len(pipelines) > 0 {
					inDevelopment = false
					break
				}
			}

			if inDevelopment {
				tt.Skip("component in development, skipping", metadataComponent)
				return
			}
			switch m.Status.Class {
			case "connector":
				assert.NotNil(tt, components.Connectors[component.Type(m.Type)], "missing connector: %s", m.Type)
			case "exporter":
				assert.NotNil(tt, components.Exporters[component.Type(m.Type)], "missing exporter: %s", m.Type)
			case "extension":
				assert.NotNil(tt, components.Extensions[component.Type(m.Type)], "missing extension: %s", m.Type)
			case "processor":
				assert.NotNil(tt, components.Processors[component.Type(m.Type)], "missing processor: %s", m.Type)
			case "receiver":
				assert.NotNil(tt, components.Receivers[component.Type(m.Type)], "missing receiver: %s", m.Type)
			}
		})
	}
}

func loadMetadata(filePath string) (metadata, error) {
	cp, err := fileprovider.New().Retrieve(context.Background(), "file:"+filePath, nil)
	if err != nil {
		return metadata{}, err
	}

	conf, err := cp.AsConf()
	if err != nil {
		return metadata{}, err
	}

	md := metadata{}
	if err := conf.Unmarshal(&md); err != nil {
		return md, err
	}

	return md, nil
}

type metadata struct {
	Type   string  `mapstructure:"type"`
	Status *status `mapstructure:"status"`
}

type status struct {
	Stability     map[string][]string `mapstructure:"stability"`
	Distributions []string            `mapstructure:"distributions"`
	Class         string              `mapstructure:"class"`
	Warnings      []string            `mapstructure:"warnings"`
}
