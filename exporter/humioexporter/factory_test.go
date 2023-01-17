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
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestCreateTracesExporter(t *testing.T) {
	// Arrange
	factory := NewFactory()
	testCases := []struct {
		desc              string
		cfg               component.Config
		wantErrorOnCreate bool
		wantErrorOnStart  bool
	}{
		{
			desc: "Valid trace configuration",
			cfg: &Config{
				Tag: TagNone,
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
				Tag: TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "\n",
				},
			},
			wantErrorOnCreate: true,
		},
		{
			desc: "Missing ingest token",
			cfg: &Config{
				Tag: TagNone,
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:8080",
				},
			},
			wantErrorOnCreate: true,
		},
		{
			desc: "Invalid client configuration",
			cfg: &Config{
				Tag: TagNone,
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
				exportertest.NewNopCreateSettings(),
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
	factory := NewFactory()
	mExp, err := factory.CreateMetricsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		factory.CreateDefaultConfig(),
	)

	require.Error(t, err)
	assert.Nil(t, mExp)
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	lExp, err := factory.CreateLogsExporter(
		context.Background(),
		exportertest.NewNopCreateSettings(),
		factory.CreateDefaultConfig(),
	)

	require.Error(t, err)
	assert.Nil(t, lExp)
}
