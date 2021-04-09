// Copyright 2019 OpenTelemetry Authors
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

package awskinesisexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.uber.org/zap/zaptest"
)

func TestCreateDefaultConfig(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, configcheck.ValidateConfig(cfg))
	assert.Equal(t, cfg.Encoding, defaultEncoding)
}

func TestCreateTracesExporter(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	r, err := createTracesExporter(context.Background(), component.ExporterCreateParams{Logger: zaptest.NewLogger(t)}, cfg)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestErrorCreateTracesExporterByInvalidEncoding(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	cfg.Encoding = ""
	r, err := createTracesExporter(context.Background(), component.ExporterCreateParams{Logger: zaptest.NewLogger(t)}, cfg)
	require.Error(t, err)
	assert.Nil(t, r)
}

func TestCreateMetricsExporter(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	r, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zaptest.NewLogger(t)}, cfg)
	require.NoError(t, err)
	assert.NotNil(t, r)
}

func TestErrorCreateMetricsExporterByInvalidEncoding(t *testing.T) {
	t.Parallel()
	cfg := createDefaultConfig().(*Config)
	cfg.Encoding = ""
	r, err := createMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zaptest.NewLogger(t)}, cfg)
	require.Error(t, err)
	assert.Nil(t, r)
}
