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

package config

import (
	"testing"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configmodels"
)

func TestConfig_Sanitize(t *testing.T) {
	type fields struct {
		ExporterSettings configmodels.ExporterSettings
		APIToken         string
		Endpoint         string
		Tags             []string
		Prefix           string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "Valid config",
			fields:  fields{APIToken: "t", Endpoint: "http://example.com"},
			wantErr: false,
		},
		{
			name:    "Missing API Token",
			fields:  fields{APIToken: "", Endpoint: "http://example.com"},
			wantErr: true,
		},
		{
			name:    "Missing Endpoint",
			fields:  fields{APIToken: "t", Endpoint: ""},
			wantErr: true,
		},
		{
			name:    "Invalid Endpoint",
			fields:  fields{APIToken: "t", Endpoint: "asdf"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{
				ExporterSettings:   tt.fields.ExporterSettings,
				APIToken:           tt.fields.APIToken,
				HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: tt.fields.Endpoint},
				Tags:               tt.fields.Tags,
				Prefix:             tt.fields.Prefix,
			}
			if err := c.Sanitize(); (err != nil) != tt.wantErr {
				t.Errorf("Config.Sanitize() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
