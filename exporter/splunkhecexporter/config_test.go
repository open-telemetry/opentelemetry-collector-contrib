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

package splunkhecexporter

import (
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e0 := cfg.Exporters["splunk_hec"]

	// Endpoint and Token do not have a default value so set them directly.
	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.Token = "00000000-0000-0000-0000-0000000000000"
	defaultCfg.Endpoint = "https://splunk:8088/services/collector"
	assert.Equal(t, defaultCfg, e0)

	expectedName := "splunk_hec/allsettings"

	e1 := cfg.Exporters[expectedName]
	expectedCfg := Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: expectedName,
		},
		Token:          "00000000-0000-0000-0000-0000000000000",
		Endpoint:       "https://splunk:8088/services/collector",
		Source:         "otel",
		SourceType:     "otel",
		Index:          "metrics",
		MaxConnections: 100,
		Timeout:        10 * time.Second,
	}
	assert.Equal(t, &expectedCfg, e1)

	te, err := factory.CreateMetricsExporter(zap.NewNop(), e1)
	require.NoError(t, err)
	require.NotNil(t, te)
}

func TestConfig_getOptionsFromConfig(t *testing.T) {
	type fields struct {
		ExporterSettings configmodels.ExporterSettings
		Endpoint         string
		Token            string
		Source           string
		SourceType       string
		Index            string
	}
	tests := []struct {
		name    string
		fields  fields
		want    *exporterOptions
		wantErr bool
	}{
		{
			name: "Test missing url",
			fields: fields{
				Token: "1234",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Test missing token",
			fields: fields{
				Endpoint: "https://example.com:8000",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Test incomplete URL",
			fields: fields{
				Token:    "1234",
				Endpoint: "https://example.com:8000",
			},
			want: &exporterOptions{
				token: "1234",
				url: &url.URL{
					Scheme: "https",
					Host:   "example.com:8000",
					Path:   "services/collector",
				},
			},
			wantErr: false,
		},
		{
			name:    "Test empty config",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ExporterSettings: tt.fields.ExporterSettings,
				Token:            tt.fields.Token,
				Endpoint:         tt.fields.Endpoint,
				Source:           tt.fields.Source,
				SourceType:       tt.fields.SourceType,
				Index:            tt.fields.Index,
			}
			got, err := cfg.getOptionsFromConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("getOptionsFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.EqualValues(t, tt.want, got)
		})
	}
}
