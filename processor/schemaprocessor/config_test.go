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

package schemaprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadingConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[config.NewComponentIDWithName(typeStr, "insert")]
	assert.Equal(t, p0, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "insert")),
	})

	p1 := cfg.Processors[config.NewComponentIDWithName(typeStr, "update")]
	assert.Equal(t, p1, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "update")),
	})

	p2 := cfg.Processors[config.NewComponentIDWithName(typeStr, "upsert")]
	assert.Equal(t, p2, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "upsert")),
	})

	p3 := cfg.Processors[config.NewComponentIDWithName(typeStr, "delete")]
	assert.Equal(t, p3, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "delete")),
	})

	p4 := cfg.Processors[config.NewComponentIDWithName(typeStr, "hash")]
	assert.Equal(t, p4, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "hash")),
	})

	p5 := cfg.Processors[config.NewComponentIDWithName(typeStr, "excludemulti")]
	assert.Equal(t, p5, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "excludemulti")),
	})

	p6 := cfg.Processors[config.NewComponentIDWithName(typeStr, "includeservices")]
	assert.Equal(t, p6, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "includeservices")),
	})

	p7 := cfg.Processors[config.NewComponentIDWithName(typeStr, "selectiveprocessing")]
	assert.Equal(t, p7, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "selectiveprocessing")),
	})

	p8 := cfg.Processors[config.NewComponentIDWithName(typeStr, "complex")]
	assert.Equal(t, p8, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "complex")),
	})

	p9 := cfg.Processors[config.NewComponentIDWithName(typeStr, "example")]
	assert.Equal(t, p9, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "example")),
	})

	p10 := cfg.Processors[config.NewComponentIDWithName(typeStr, "regexp")]
	assert.Equal(t, p10, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "regexp")),
	})
}
