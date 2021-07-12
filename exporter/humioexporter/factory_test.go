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

package humioexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

func newHumioFactory(t *testing.T) component.ExporterFactory {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory

	return factory
}

func TestCreateTracesExporter(t *testing.T) {
	// Arrange
	factory := newHumioFactory(t)
	testCases := []struct {
		desc              string
		cfg               config.Exporter
		wantErrorOnCreate bool
		wantErrorOnStart  bool
	}{
		{
			desc: "Valid trace configuration",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8080",
				},
				Traces: TracesConfig{
					IngestToken: "00000000-0000-0000-0000-0000000000000",
				},
			},
			wantErrorOnCreate: false,
		},
		{
			desc: "Unsanitizable trace configuration",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "\n",
				},
			},
			wantErrorOnCreate: true,
		},
		{
			desc: "Missing ingest token",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8080",
				},
			},
			wantErrorOnCreate: true,
		},
		{
			desc: "Invalid client configuration",
			cfg: &Config{
				ExporterSettings: config.NewExporterSettings(config.NewID(typeStr)),
				Tag:              TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8080",
					TLSSetting: configtls.TLSClientSetting{
						TLSSetting: configtls.TLSSetting{
							CertFile: "",
							KeyFile:  "key.key",
						},
					},
				},
				Traces: TracesConfig{
					IngestToken: "00000000-0000-0000-0000-0000000000000",
				},
			},
			wantErrorOnCreate: false,
			wantErrorOnStart:  true,
		},
		{
			desc:              "Missing configuration",
			cfg:               nil,
			wantErrorOnCreate: true,
		},
	}

	// Act / Assert
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			exp, err := factory.CreateTracesExporter(
				context.Background(),
				componenttest.NewNopExporterCreateSettings(),
				tC.cfg,
			)

			if (err != nil) != tC.wantErrorOnCreate {
				t.Errorf("CreateTracesExporter() error = %v, wantErr %v", err, tC.wantErrorOnCreate)
			}

			if (err == nil) && (exp == nil) {
				t.Error("No trace exporter created despite no errors")
			}

			if exp != nil {
				err = exp.Start(context.Background(), componenttest.NewNopHost())
				if (err != nil) != tC.wantErrorOnStart {
					t.Errorf("CreateTracesExporter() error = %v, wantErr %v", err, tC.wantErrorOnStart)
				}
			}
		})
	}
}

func TestCreateMetricsExporter(t *testing.T) {
	factory := newHumioFactory(t)
	mExp, err := factory.CreateMetricsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		factory.CreateDefaultConfig(),
	)

	require.Error(t, err)
	assert.Nil(t, mExp)
}

func TestCreateLogsExporter(t *testing.T) {
	factory := newHumioFactory(t)
	lExp, err := factory.CreateLogsExporter(
		context.Background(),
		componenttest.NewNopExporterCreateSettings(),
		factory.CreateDefaultConfig(),
	)

	require.Error(t, err)
	assert.Nil(t, lExp)
}
