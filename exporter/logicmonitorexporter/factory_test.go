// Copyright The OpenTelemetry Authors
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

package logicmonitorexporter

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

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ExporterSettings:    config.NewExporterSettings(component.NewID(typeStr)),
		LogBatchingEnabled:  DefaultLogBatchingEnabled,
		LogBatchingInterval: DefaultLogBatchingInterval,
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateTracesExporter(t *testing.T) {
	endpoint := "http://" + GetAvailableLocalAddress(t)
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Error scenario",
			config: Config{
				URL: "&(*)8#RE/48df$#rest",
			},
			wantErr: true,
		},
		{
			name: "Non Error scenario",
			config: Config{
				ExporterSettings: config.NewExporterSettings(component.NewID(typeStr)),
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: endpoint,
					TLSSetting: configtls.TLSClientSetting{
						Insecure: false,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
			factory := NewFactory()
			set := componenttest.NewNopExporterCreateSettings()
			_, err := factory.CreateTracesExporter(context.Background(), set, &tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateTracesExporter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateLogsExporter(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	set := componenttest.NewNopExporterCreateSettings()
	t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
	oexp, err := factory.CreateLogsExporter(context.Background(), set, cfg)
	require.Nil(t, err)
	require.NotNil(t, oexp)
}
