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

package cloudfoundryreceiver

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id           config.ComponentID
		expected     config.Receiver
		errorMessage string
	}{
		{
			id: config.NewComponentIDWithName(typeStr, "one"),
			expected: &Config{
				ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
				RLPGateway: RLPGatewayConfig{
					HTTPClientSettings: confighttp.HTTPClientSettings{
						Endpoint: "https://log-stream.sys.example.internal",
						TLSSetting: configtls.TLSClientSetting{
							InsecureSkipVerify: true,
						},
						Timeout: time.Second * 20,
					},
					ShardID: "otel-test",
				},
				UAA: UAAConfig{
					LimitedHTTPClientSettings: LimitedHTTPClientSettings{
						Endpoint: "https://uaa.sys.example.internal",
						TLSSetting: LimitedTLSClientSetting{
							InsecureSkipVerify: true,
						},
					},
					Username: "admin",
					Password: "test",
				},
			},
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "empty"),
			errorMessage: "UAA password not specified",
		},
		{
			id:           config.NewComponentIDWithName(typeStr, "invalid"),
			errorMessage: "failed to parse rlp_gateway.endpoint as url: parse \"https://[invalid\": missing ']' in host",
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalReceiver(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, cfg.Validate(), tt.errorMessage)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestInvalidConfigValidation(t *testing.T) {
	configuration := loadSuccessfulConfig(t)
	configuration.RLPGateway.Endpoint = "https://[invalid"
	require.Error(t, configuration.Validate())

	configuration = loadSuccessfulConfig(t)
	configuration.UAA.Username = ""
	require.Error(t, configuration.Validate())

	configuration = loadSuccessfulConfig(t)
	configuration.UAA.Password = ""
	require.Error(t, configuration.Validate())

	configuration = loadSuccessfulConfig(t)
	configuration.UAA.Endpoint = "https://[invalid"
	require.Error(t, configuration.Validate())
}

func TestHTTPConfigurationStructConsistency(t *testing.T) {
	// LimitedHTTPClientSettings must have the same structure as HTTPClientSettings, but without the fields that the UAA
	// library does not support.
	checkTypeFieldMatch(t, "Endpoint", reflect.TypeOf(LimitedHTTPClientSettings{}), reflect.TypeOf(confighttp.HTTPClientSettings{}))
	checkTypeFieldMatch(t, "TLSSetting", reflect.TypeOf(LimitedHTTPClientSettings{}), reflect.TypeOf(confighttp.HTTPClientSettings{}))
	checkTypeFieldMatch(t, "InsecureSkipVerify", reflect.TypeOf(LimitedTLSClientSetting{}), reflect.TypeOf(configtls.TLSClientSetting{}))
}

func loadSuccessfulConfig(t *testing.T) *Config {
	configuration := &Config{
		RLPGateway: RLPGatewayConfig{
			HTTPClientSettings: confighttp.HTTPClientSettings{
				Endpoint: "https://log-stream.sys.example.internal",
				Timeout:  time.Second * 20,
				TLSSetting: configtls.TLSClientSetting{
					InsecureSkipVerify: true,
				},
			},
			ShardID: "otel-test",
		},
		UAA: UAAConfig{
			LimitedHTTPClientSettings: LimitedHTTPClientSettings{
				Endpoint: "https://uaa.sys.example.internal",
				TLSSetting: LimitedTLSClientSetting{
					InsecureSkipVerify: true,
				},
			},
			Username: "admin",
			Password: "test",
		},
	}

	require.NoError(t, configuration.Validate())
	return configuration
}

func checkTypeFieldMatch(t *testing.T, fieldName string, localType reflect.Type, standardType reflect.Type) {
	localField, localFieldPresent := localType.FieldByName(fieldName)
	standardField, standardFieldPresent := standardType.FieldByName(fieldName)

	require.True(t, localFieldPresent, "field %s present in local type", fieldName)
	require.True(t, standardFieldPresent, "field %s present in standard type", fieldName)
	require.Equal(t, localField.Tag, standardField.Tag, "field %s tag match", fieldName)
}
