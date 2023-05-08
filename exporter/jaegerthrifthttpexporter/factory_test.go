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

package jaegerthrifthttpexporter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateInstanceViaFactory(t *testing.T) {
	cfg := createDefaultConfig()
	params := exportertest.NewNopCreateSettings()
	// Endpoint doesn't have a default value so set it directly.
	expCfg := cfg.(*Config)
	expCfg.HTTPClientSettings.Endpoint = "http://jaeger.example.com:12345/api/traces"
	exp, err := createTracesExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	assert.NoError(t, exp.Shutdown(context.Background()))
}

func TestFactory_CreateTracesExporter(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: "http://jaeger.example.com/api/traces",
			Headers: map[string]configopaque.String{
				"added-entry": "added value",
				"dot.test":    "test",
			},
			Timeout: 2 * time.Second,
		},
	}

	params := exportertest.NewNopCreateSettings()
	te, err := createTracesExporter(context.Background(), params, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}
