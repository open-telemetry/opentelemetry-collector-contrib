// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbyattrsprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)
	factory := NewFactory()
	factories.Processors[typeStr] = factory

	factories.Processors["batch"] = batchprocessor.NewFactory()
	factories.Processors["groupbytrace"] = groupbytraceprocessor.NewFactory()

	require.NoError(t, err)

	err = configtest.CheckConfigStruct(factory.CreateDefaultConfig())
	require.NoError(t, err)

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	groupingConf := cfg.Processors[config.NewComponentIDWithName(typeStr, "grouping")]
	assert.Equal(t, groupingConf,
		&Config{
			ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "grouping")),
			GroupByKeys:       []string{"key1", "key2"},
		})

	compactionConf := cfg.Processors[config.NewComponentIDWithName(typeStr, "compaction")]
	assert.Equal(t, compactionConf,
		&Config{
			ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "compaction")),
			GroupByKeys:       []string{},
		})
}
