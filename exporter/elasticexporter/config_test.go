// Copyright 2020, OpenTelemetry Authors
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

package elasticexporter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Exporters), 2)

	r0 := cfg.Exporters["elastic"]
	assert.Equal(t, r0, factory.CreateDefaultConfig())

	r1 := cfg.Exporters["elastic/customname"].(*Config)
	assert.Equal(t, r1, &Config{
		ExporterSettings: configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "elastic/customname"},
		APMServerURL:     "https://elastic.example.com",
		APIKey:           "RTNxMjlXNEJt",
		SecretToken:      "hunter2",
	})
}

func TestConfigValidate(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	_, err := factory.CreateTraceExporter(context.Background(), params, cfg)
	require.Error(t, err)
	assert.EqualError(t, err, "cannot configure Elastic APM trace exporter: invalid config: APMServerURL must be specified")

	_, err = factory.CreateMetricsExporter(context.Background(), params, cfg)
	require.Error(t, err)
	assert.EqualError(t, err, "cannot configure Elastic APM metrics exporter: invalid config: APMServerURL must be specified")

	cfg.APMServerURL = "foo"
	_, err = factory.CreateTraceExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	_, err = factory.CreateMetricsExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
}

func TestConfigAuth(t *testing.T) {
	testAuth(t, "", "hunter2", "Bearer hunter2")
	testAuth(t, "hunter2", "", "ApiKey hunter2")
}

func testAuth(t *testing.T, apiKey, secretToken, expectedAuthorization string) {
	factory := NewFactory()
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.APIKey = apiKey
	cfg.SecretToken = secretToken

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != expectedAuthorization {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, "Expected Authorization=%s, got %s\n", expectedAuthorization, auth)
		}
	}))
	defer srv.Close()
	cfg.APMServerURL = srv.URL

	te, err := factory.CreateTraceExporter(context.Background(), params, cfg)
	assert.NoError(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	traces := pdata.NewTraces()
	resourceSpans := traces.ResourceSpans()
	resourceSpans.Resize(1)
	resourceSpans.At(0).InitEmpty()
	resourceSpans.At(0).InstrumentationLibrarySpans().Resize(1)
	resourceSpans.At(0).InstrumentationLibrarySpans().At(0).Spans().Resize(1)
	span := resourceSpans.At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
	span.SetName("foobar")
	assert.NoError(t, te.ConsumeTraces(context.Background(), traces))
}
