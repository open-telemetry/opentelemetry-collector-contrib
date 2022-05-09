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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		BaseConfig: stanza.BaseConfig{
			Operators: stanza.OperatorConfigs{
				map[string]interface{}{
					"type":  "regex_parser",
					"regex": "^(?P<time>\\d{4}-\\d{2}-\\d{2}) (?P<sev>[A-Z]*) (?P<msg>.*)$",
					"severity": map[string]interface{}{
						"parse_from": "body.sev",
					},
					"timestamp": map[string]interface{}{
						"layout":     "%Y-%m-%d",
						"parse_from": "body.time",
					},
				},
			},
			Converter: stanza.ConverterConfig{
				MaxFlushCount: 500,
				FlushInterval: 13 * time.Millisecond,
			},
		},
	}

	tp, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)
}

func TestInvalidOperators(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		BaseConfig: stanza.BaseConfig{
			Operators: stanza.OperatorConfigs{
				map[string]interface{}{
					"type": "nonsense",
				},
			},
		},
	}

	_, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, nil)
	assert.Error(t, err)
}
