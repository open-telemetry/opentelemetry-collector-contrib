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
	"context"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e0 := cfg.Exporters[config.NewID(typeStr)]

	// Endpoint and Token do not have a default value so set them directly.
	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.Token = "00000000-0000-0000-0000-0000000000000"
	defaultCfg.Endpoint = "https://splunk:8088/services/collector"
	assert.Equal(t, defaultCfg, e0)

	e1 := cfg.Exporters[config.NewIDWithName(typeStr, "allsettings")]
	expectedCfg := Config{
		ExporterSettings:     config.NewExporterSettings(config.NewIDWithName(typeStr, "allsettings")),
		Token:                "00000000-0000-0000-0000-0000000000000",
		Endpoint:             "https://splunk:8088/services/collector",
		Source:               "otel",
		SourceType:           "otel",
		Index:                "metrics",
		SplunkAppName:        "OpenTelemetry-Collector Splunk Exporter",
		SplunkAppVersion:     "v0.0.1",
		MaxConnections:       100,
		MaxContentLengthLogs: 2 * 1024 * 1024,
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 10 * time.Second,
		},
		RetrySettings: exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 10 * time.Second,
			MaxInterval:     1 * time.Minute,
			MaxElapsedTime:  10 * time.Minute,
		},
		QueueSettings: exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 2,
			QueueSize:    10,
		},
		TLSSetting: configtls.TLSClientSetting{
			TLSSetting: configtls.TLSSetting{
				CAFile:   "",
				CertFile: "",
				KeyFile:  "",
			},
			InsecureSkipVerify: false,
		},
	}
	assert.Equal(t, &expectedCfg, e1)

	params := componenttest.NewNopExporterCreateSettings()
	te, err := factory.CreateMetricsExporter(context.Background(), params, e1)
	require.NoError(t, err)
	require.NotNil(t, te)
}

func TestConfig_getOptionsFromConfig(t *testing.T) {
	type fields struct {
		Endpoint             string
		Token                string
		Source               string
		SourceType           string
		Index                string
		MaxContentLengthLogs uint
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
		{
			name: "Test max content length logs greater than limit",
			fields: fields{
				Token:                "1234",
				Endpoint:             "https://example.com:8000",
				MaxContentLengthLogs: maxContentLengthLogsLimit + 1,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Token:                tt.fields.Token,
				Endpoint:             tt.fields.Endpoint,
				Source:               tt.fields.Source,
				SourceType:           tt.fields.SourceType,
				Index:                tt.fields.Index,
				MaxContentLengthLogs: tt.fields.MaxContentLengthLogs,
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
