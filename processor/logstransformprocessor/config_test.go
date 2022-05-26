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

package logstransformprocessor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factories.Processors[typeStr] = NewFactory()

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	assert.Equal(t, cfg.Processors[config.NewComponentID(typeStr)], &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		BaseConfig: stanza.BaseConfig{
			ReceiverSettings: config.ReceiverSettings{},
			Operators: stanza.OperatorConfigs{
				map[string]interface{}{
					"type":  "regex_parser",
					"regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$",
					"severity": map[string]interface{}{
						"parse_from": "attributes.sev",
					},
					"timestamp": map[string]interface{}{
						"layout":     "%Y-%m-%d %H:%M:%S",
						"parse_from": "attributes.time",
					},
				},
			},
			Converter: stanza.ConverterConfig{
				MaxFlushCount: 100,
				FlushInterval: 100 * time.Millisecond,
			},
		},
	})
}
