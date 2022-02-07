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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	require.Len(t, cfg.Receivers, 2)

	r0 := cfg.Receivers[config.NewComponentID(typeStr)]
	defaultConfig := factory.CreateDefaultConfig().(*Config)
	defaultConfig.UAA.Password = "test"
	assert.Equal(t, defaultConfig, r0)

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "one")].(*Config)
	assert.Equal(t,
		&Config{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "one")),
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
		}, r1)
}

func TestLoadInvalidConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	_, err = servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalid.yaml"), factories)

	require.Error(t, err)
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
