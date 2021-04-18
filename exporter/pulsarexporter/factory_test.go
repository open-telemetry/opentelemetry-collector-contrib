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

package pulsarexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
	assert.Equal(t, defaultURL, cfg.URL)
	assert.Equal(t, "", cfg.Topic)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory(
		WithAddTracesMarshallers(map[string]TracesMarshaller{}),
		WithAddMetricsMarshallers(map[string]MetricsMarshaller{}),
		WithAddLogsMarshallers(map[string]LogsMarshaller{}),
	)
	ctx := context.Background()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	cfg := &Config{}
	t.Run("CreateTracesExporter", func(t *testing.T) {
		exp, err := factory.CreateTracesExporter(ctx, params, cfg)
		assert.Nil(t, err)
		assert.NotNil(t, exp)
	})
	t.Run("CreateMetricsExporter", func(t *testing.T) {
		exp, err := factory.CreateMetricsExporter(ctx, params, cfg)
		assert.Nil(t, err)
		assert.NotNil(t, exp)
	})
	t.Run("CreateMetricsExporter", func(t *testing.T) {
		exp, err := factory.CreateLogsExporter(ctx, params, cfg)
		assert.Nil(t, err)
		assert.NotNil(t, exp)
	})
}
