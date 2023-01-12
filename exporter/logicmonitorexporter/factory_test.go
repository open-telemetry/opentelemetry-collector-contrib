// Copyright  The OpenTelemetry Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
	}, cfg, "failed to create default config")

	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

// TODO: Lines commented out until implementation is available
// func TestCreateTracesExporter(t *testing.T) {
// 	tests := []struct {
// 		name    string
// 		config  Config
// 		wantErr bool
// 	}{
// 		{
// 			name: "Error scenario",
// 			config: Config{
// 				HTTPClientSettings: confighttp.HTTPClientSettings{
// 					Endpoint: "example.com",
// 				},
// 			},
// 			wantErr: true,
// 		},
// 		{
// 			name: "Non Error scenario",
// 			config: Config{
// 				HTTPClientSettings: confighttp.HTTPClientSettings{
// 					Endpoint: "http://example.com",
// 					TLSSetting: configtls.TLSClientSetting{
// 						Insecure: false,
// 					},
// 				},
// 			},
// 			wantErr: false,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
// 			factory := NewFactory()
// 			set := exportertest.NewNopCreateSettings()
// 			_, err := factory.CreateTracesExporter(context.Background(), set, &tt.config)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("CreateTracesExporter() error = %v, wantErr %v", err, tt.wantErr)
// 			}
// 		})
// 	}
// }

// func TestCreateLogsExporter(t *testing.T) {
// 	tests := []struct {
// 		name         string
// 		config       Config
// 		shouldError  bool
// 		errorMessage string
// 	}{
// 		{
// 			name: "valid config",
// 			config: Config{
// 				HTTPClientSettings: confighttp.HTTPClientSettings{
// 					Endpoint: "http://" + testutil.GetAvailableLocalAddress(t),
// 				},
// 			},
// 			shouldError: false,
// 		},
// 		{
// 			name: "invalid config",
// 			config: Config{
// 				HTTPClientSettings: confighttp.HTTPClientSettings{
// 					Endpoint: "",
// 				},
// 			},
// 			shouldError: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			factory := NewFactory()
// 			cfg := factory.CreateDefaultConfig().(*Config)
// 			set := exportertest.NewNopCreateSettings()
// 			t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
// 			oexp, err := factory.CreateLogsExporter(context.Background(), set, cfg)
// 			if (err != nil) != tt.shouldError {
// 				t.Errorf("CreateLogsExporter() error = %v, shouldError %v", err, tt.shouldError)
// 				return
// 			}
// 			if tt.shouldError {
// 				assert.Error(t, err)
// 				if len(tt.errorMessage) != 0 {
// 					assert.Equal(t, tt.errorMessage, err.Error())
// 				}
// 				return
// 			}
// 			assert.NoError(t, err)
// 			assert.NotNil(t, oexp)
// 		})
// 	}
// }
