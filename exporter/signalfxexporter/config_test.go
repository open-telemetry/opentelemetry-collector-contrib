// Copyright 2019, OpenTelemetry Authors
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

package signalfxexporter

import (
	"context"
	"net/url"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e0 := cfg.Exporters["signalfx"]

	// Realm doesn't have a default value so set it directly.
	defaultCfg := factory.CreateDefaultConfig().(*Config)
	defaultCfg.Realm = "ap0"
	assert.Equal(t, defaultCfg, e0)

	expectedName := "signalfx/allsettings"

	e1 := cfg.Exporters[expectedName]
	expectedCfg := Config{
		ExporterSettings: configmodels.ExporterSettings{
			TypeVal: configmodels.Type(typeStr),
			NameVal: expectedName,
		},
		AccessToken: "testToken",
		Realm:       "us1",
		Headers: map[string]string{
			"added-entry": "added value",
			"dot.test":    "test",
		},
		TimeoutSettings: exporterhelper.TimeoutSettings{
			Timeout: 2 * time.Second,
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
		}, AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
			AccessTokenPassthrough: false,
		},
		SendCompatibleMetrics: true,
		TranslationRules: []translation.Rule{
			{
				Action: translation.ActionRenameDimensionKeys,
				Mapping: map[string]string{
					"k8s.cluster.name": "kubernetes_cluster",
				},
			},
		},
		DeltaTranslationTTL: 3600,
	}
	assert.Equal(t, &expectedCfg, e1)

	te, err := factory.CreateMetricsExporter(context.Background(), component.ExporterCreateParams{Logger: zap.NewNop()}, e1)
	require.NoError(t, err)
	require.NotNil(t, te)
}

func TestConfig_getOptionsFromConfig(t *testing.T) {
	type fields struct {
		ExporterSettings      configmodels.ExporterSettings
		AccessToken           string
		Realm                 string
		IngestURL             string
		APIURL                string
		Timeout               time.Duration
		Headers               map[string]string
		SendCompatibleMetrics bool
		TranslationRules      []translation.Rule
		SyncHostMetadata      bool
	}
	tests := []struct {
		name    string
		fields  fields
		want    *exporterOptions
		wantErr bool
	}{
		{
			name: "Test URL overrides",
			fields: fields{
				Realm:       "us0",
				AccessToken: "access_token",
				IngestURL:   "https://ingest.us1.signalfx.com/",
				APIURL:      "https://api.us1.signalfx.com/",
			},
			want: &exporterOptions{
				ingestURL: &url.URL{
					Scheme: "https",
					Host:   "ingest.us1.signalfx.com",
					Path:   "/",
				},
				apiURL: &url.URL{
					Scheme: "https",
					Host:   "api.us1.signalfx.com",
					Path:   "/",
				},
				httpTimeout: 5 * time.Second,
				token:       "access_token",
			},
			wantErr: false,
		},
		{
			name: "Test URL from Realm",
			fields: fields{
				Realm:       "us0",
				AccessToken: "access_token",
				Timeout:     10 * time.Second,
			},
			want: &exporterOptions{
				ingestURL: &url.URL{
					Scheme: "https",
					Host:   "ingest.us0.signalfx.com",
					Path:   "",
				},
				apiURL: &url.URL{
					Scheme: "https",
					Host:   "api.us0.signalfx.com",
				},
				httpTimeout: 10 * time.Second,
				token:       "access_token",
			},
			wantErr: false,
		},
		{
			name: "Test empty realm and API URL",
			fields: fields{
				AccessToken: "access_token",
				Timeout:     10 * time.Second,
				IngestURL:   "https://ingest.us1.signalfx.com/",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Test empty realm and Ingest URL",
			fields: fields{
				AccessToken: "access_token",
				Timeout:     10 * time.Second,
				APIURL:      "https://api.us1.signalfx.com/",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Test invalid URLs",
			fields: fields{
				AccessToken: "access_token",
				Timeout:     10 * time.Second,
				APIURL:      "https://api us1 signalfx com/",
				IngestURL:   "https://api us1 signalfx com/",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Test empty config",
			want:    nil,
			wantErr: true,
		},
		{
			name: "Test invalid translation rules",
			fields: fields{
				Realm:                 "us0",
				AccessToken:           "access_token",
				SendCompatibleMetrics: true,
				TranslationRules: []translation.Rule{
					{
						Action: translation.ActionRenameDimensionKeys,
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				ExporterSettings: tt.fields.ExporterSettings,
				AccessToken:      tt.fields.AccessToken,
				Realm:            tt.fields.Realm,
				IngestURL:        tt.fields.IngestURL,
				APIURL:           tt.fields.APIURL,
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: tt.fields.Timeout,
				},
				Headers:               tt.fields.Headers,
				SendCompatibleMetrics: tt.fields.SendCompatibleMetrics,
				TranslationRules:      tt.fields.TranslationRules,
				SyncHostMetadata:      tt.fields.SyncHostMetadata,
			}
			got, err := cfg.getOptionsFromConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("getOptionsFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			require.Equal(t, tt.want, got)
		})
	}
}
