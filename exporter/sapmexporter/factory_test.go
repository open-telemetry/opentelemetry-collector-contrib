// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sapmexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateExporter(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, "sapm", string(factory.Type()))

	cfg := factory.CreateDefaultConfig()
	eCfg := cfg.(*Config)
	eCfg.Endpoint = "http://local"
	eCfg.Correlation.Endpoint = "http://local"
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := factory.CreateTraceExporter(context.Background(), params, eCfg)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := factory.CreateMetricsExporter(context.Background(), params, eCfg)
	assert.Error(t, err)
	assert.Nil(t, me)
}

func TestCreateExporterWithoutAPIEndpoint(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Endpoint = "http://local"
	cfg.Correlation.Enabled = true
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	te, err := factory.CreateTraceExporter(context.Background(), params, cfg)
	assert.Nil(t, te)
	assert.EqualError(t, err, "`correlation.endpoint` must be set when `correlation.enabled` is true")
}
