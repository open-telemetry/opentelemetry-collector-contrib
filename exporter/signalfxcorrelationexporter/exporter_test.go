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

package signalfxcorrelationexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestCreateTraceExporter(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost"},
		ExporterSettings:   configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "signalfx_correlation/configured"},
		AccessToken:        "abcd1234",
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := newCorrExporter(config, params)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")

	assert.NoError(t, te.Shutdown(context.Background()), "trace exporter shutdown failed")
}

func TestCreateTraceExporterWithInvalidConfig(t *testing.T) {
	config := &Config{}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}
	te, err := newTraceExporter(config, params)
	require.Error(t, err)
	assert.Nil(t, te)
}

func TestExporterConsumeTraces(t *testing.T) {
	config := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: "http://localhost"},
		ExporterSettings:   configmodels.ExporterSettings{TypeVal: configmodels.Type(typeStr), NameVal: "signalfx_correlation/configured"},
		AccessToken:        "abcd1234",
	}
	params := component.ExporterCreateParams{Logger: zap.NewNop()}

	te, err := newTraceExporter(config, params)
	assert.Nil(t, err)
	assert.NotNil(t, te, "failed to create trace exporter")
	defer te.Shutdown(context.Background())

	assert.NoError(t, te.ConsumeTraces(context.Background(), pdata.NewTraces()))
}
