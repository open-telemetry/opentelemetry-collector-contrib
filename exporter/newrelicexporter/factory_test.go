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

package newrelicexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.uber.org/zap"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, configcheck.ValidateConfig(cfg))

	nrCfg, ok := cfg.(*Config)
	require.True(t, ok, "invalid Config: %#v", cfg)
	assert.Equal(t, nrCfg.Timeout, time.Second*15)
}

func TestCreateExporter(t *testing.T) {
	cfg := createDefaultConfig()
	nrConfig := cfg.(*Config)
	nrConfig.APIKey = "a1b2c3d4"
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := createTraceExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	me, err := createMetricsExporter(context.Background(), params, nrConfig)
	assert.Nil(t, err)
	assert.NotNil(t, me, "failed to create metrics exporter")
}

func TestCreateTraceExporterError(t *testing.T) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createTraceExporter(context.Background(), params, nil)
	assert.Error(t, err)
}

func TestCreateMetricsExporterError(t *testing.T) {
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	_, err := createMetricsExporter(context.Background(), params, nil)
	assert.Error(t, err)
}
